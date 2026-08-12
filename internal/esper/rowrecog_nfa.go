package esper

// This file contains the small NFA used by the runtime-wide MatchRecognize
// state pool. The result matcher remains history based (it is also used by
// iterate-only and snapshot evaluation), while the state pool needs the same
// node/edge lifecycle as Esper's RowRecogNFAView. Keeping that accounting NFA
// separate makes the resource policy observable without changing result
// ranking or measure evaluation.

const maxRowRecogStatePoolNFANodes = 4096

type rowRecogStatePoolNFANode struct {
	id       int
	variable string
	next     []*rowRecogStatePoolNFANode
	terminal bool
}

type rowRecogStatePoolNFAFragment struct {
	starts      []*rowRecogStatePoolNFANode
	ends        []*rowRecogStatePoolNFANode
	passthrough bool
}

type rowRecogStatePoolNFA struct {
	starts []*rowRecogStatePoolNFANode
	nodes  []*rowRecogStatePoolNFANode
}

type rowRecogNFAPath struct {
	node     *rowRecogStatePoolNFANode
	captures map[string][]Event
}

type rowRecogStatePoolNFACompiler struct {
	nextID int
	nodes  []*rowRecogStatePoolNFANode
	failed bool
}

func rowRecogStatePoolNFAFor(definition *rowRecogDefinition) *rowRecogStatePoolNFA {
	if definition == nil {
		return nil
	}
	definition.statePoolNFAOnce.Do(func() {
		definition.statePoolNFA = compileRowRecogStatePoolNFA(definition.pattern)
	})
	return definition.statePoolNFA
}

func compileRowRecogStatePoolNFA(pattern RowPattern) *rowRecogStatePoolNFA {
	compiler := &rowRecogStatePoolNFACompiler{}
	fragment := compiler.compile(pattern)
	if compiler.failed || len(fragment.starts) == 0 || len(fragment.ends) == 0 {
		return nil
	}
	for _, end := range fragment.ends {
		end.terminal = true
	}
	return &rowRecogStatePoolNFA{
		starts: append([]*rowRecogStatePoolNFANode(nil), fragment.starts...),
		nodes:  append([]*rowRecogStatePoolNFANode(nil), compiler.nodes...),
	}
}

func (c *rowRecogStatePoolNFACompiler) compile(pattern RowPattern) rowRecogStatePoolNFAFragment {
	if c == nil || c.failed {
		return rowRecogStatePoolNFAFragment{}
	}
	minimum, maximum := rowPatternBounds(pattern)
	if minimum == 1 && maximum == 1 {
		return c.compileBase(pattern)
	}
	base := pattern
	base.quantified = false
	base.minimum = 1
	base.maximum = 1
	return c.compileQuantified(base, minimum, maximum)
}

func (c *rowRecogStatePoolNFACompiler) compileBase(pattern RowPattern) rowRecogStatePoolNFAFragment {
	if c == nil || c.failed {
		return rowRecogStatePoolNFAFragment{}
	}
	switch pattern.kind {
	case rowPatternVariable:
		node := c.newNode(pattern.name)
		if node == nil {
			return rowRecogStatePoolNFAFragment{}
		}
		return rowRecogStatePoolNFAFragment{
			starts: []*rowRecogStatePoolNFANode{node},
			ends:   []*rowRecogStatePoolNFANode{node},
		}
	case rowPatternSequence:
		fragments := make([]rowRecogStatePoolNFAFragment, 0, len(pattern.parts))
		for _, part := range pattern.parts {
			fragments = append(fragments, c.compile(part))
		}
		return c.concatenate(fragments)
	case rowPatternAlternation:
		return c.alternate(pattern.parts)
	case rowPatternPermutation:
		orders := rowPatternPermutationOrders(pattern.parts)
		fragments := make([]rowRecogStatePoolNFAFragment, 0, len(orders))
		for _, order := range orders {
			parts := make([]rowRecogStatePoolNFAFragment, 0, len(order))
			for _, part := range order {
				parts = append(parts, c.compile(part))
			}
			fragments = append(fragments, c.concatenate(parts))
		}
		return c.alternateFragments(fragments)
	default:
		c.failed = true
		return rowRecogStatePoolNFAFragment{}
	}
}

func (c *rowRecogStatePoolNFACompiler) compileQuantified(base RowPattern, minimum, maximum int) rowRecogStatePoolNFAFragment {
	if c == nil || c.failed || minimum < 0 || (maximum > 0 && minimum > maximum) {
		if c != nil {
			c.failed = true
		}
		return rowRecogStatePoolNFAFragment{}
	}

	if maximum == 0 {
		// Esper represents A+ and A* with one looping NFA node/strand. For
		// larger lower bounds it expands the mandatory copies followed by a
		// zero-to-many copy; preserve that distinction for state counts.
		if minimum == 0 || minimum == 1 {
			fragment := c.compileBase(base)
			c.connect(fragment.ends, fragment.starts)
			fragment.passthrough = fragment.passthrough || minimum == 0
			return fragment
		}
		mandatory := make([]rowRecogStatePoolNFAFragment, 0, minimum+1)
		for index := 0; index < minimum; index++ {
			mandatory = append(mandatory, c.compileBase(base))
		}
		tail := c.compileBase(base)
		c.connect(tail.ends, tail.starts)
		tail.passthrough = true
		mandatory = append(mandatory, tail)
		return c.concatenate(mandatory)
	}

	parts := make([]rowRecogStatePoolNFAFragment, 0, maximum)
	for index := 0; index < maximum; index++ {
		fragment := c.compileBase(base)
		if index >= minimum {
			fragment.passthrough = true
		}
		parts = append(parts, fragment)
	}
	return c.concatenate(parts)
}

func (c *rowRecogStatePoolNFACompiler) alternate(parts []RowPattern) rowRecogStatePoolNFAFragment {
	fragments := make([]rowRecogStatePoolNFAFragment, 0, len(parts))
	for _, part := range parts {
		fragments = append(fragments, c.compile(part))
	}
	return c.alternateFragments(fragments)
}

func (c *rowRecogStatePoolNFACompiler) alternateFragments(fragments []rowRecogStatePoolNFAFragment) rowRecogStatePoolNFAFragment {
	if len(fragments) == 0 {
		c.failed = true
		return rowRecogStatePoolNFAFragment{}
	}
	result := rowRecogStatePoolNFAFragment{}
	for _, fragment := range fragments {
		result.starts = appendUniqueRowRecogNFAStates(result.starts, fragment.starts...)
		result.ends = appendUniqueRowRecogNFAStates(result.ends, fragment.ends...)
		result.passthrough = result.passthrough || fragment.passthrough
	}
	return result
}

func (c *rowRecogStatePoolNFACompiler) concatenate(fragments []rowRecogStatePoolNFAFragment) rowRecogStatePoolNFAFragment {
	if len(fragments) == 0 {
		c.failed = true
		return rowRecogStatePoolNFAFragment{}
	}
	for index := len(fragments) - 1; index >= 1; index-- {
		current := fragments[index]
		for prior := index - 1; prior >= 0; prior-- {
			c.connect(fragments[prior].ends, current.starts)
			if !fragments[prior].passthrough {
				break
			}
		}
	}

	result := rowRecogStatePoolNFAFragment{passthrough: true}
	for _, fragment := range fragments {
		result.starts = appendUniqueRowRecogNFAStates(result.starts, fragment.starts...)
		if !fragment.passthrough {
			result.passthrough = false
			break
		}
	}
	for index := len(fragments) - 1; index >= 0; index-- {
		fragment := fragments[index]
		result.ends = appendUniqueRowRecogNFAStates(result.ends, fragment.ends...)
		if !fragment.passthrough {
			break
		}
	}
	return result
}

func (c *rowRecogStatePoolNFACompiler) connect(from, to []*rowRecogStatePoolNFANode) {
	for _, source := range from {
		if source == nil {
			continue
		}
		source.next = appendUniqueRowRecogNFAStates(source.next, to...)
	}
}

func (c *rowRecogStatePoolNFACompiler) newNode(variable string) *rowRecogStatePoolNFANode {
	if c == nil || c.failed || c.nextID >= maxRowRecogStatePoolNFANodes {
		if c != nil {
			c.failed = true
		}
		return nil
	}
	node := &rowRecogStatePoolNFANode{id: c.nextID, variable: variable}
	c.nextID++
	c.nodes = append(c.nodes, node)
	return node
}

func appendUniqueRowRecogNFAStates(dst []*rowRecogStatePoolNFANode, states ...*rowRecogStatePoolNFANode) []*rowRecogStatePoolNFANode {
	for _, state := range states {
		if state == nil {
			continue
		}
		found := false
		for _, existing := range dst {
			if existing == state {
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, state)
		}
	}
	return dst
}
