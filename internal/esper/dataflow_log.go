package esper

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
)

// DataflowLogFormat selects the representation rendered by a structured
// LogSink. The zero value uses the Esper-compatible summary representation.
type DataflowLogFormat string

const (
	DataflowLogSummary DataflowLogFormat = "summary"
	DataflowLogJSON    DataflowLogFormat = "json"
	DataflowLogXML     DataflowLogFormat = "xml"
)

// DataflowLogWriter receives one rendered LogSink line. A nil Writer writes to
// stdout, while an application can supply a logger, buffer or test collector.
type DataflowLogWriter func(context.Context, string) error

// DataflowLogSinkOptions configures the structured LogSink form. LineFeed
// defaults to true, matching Esper. Set LineFeedSet when explicitly disabling
// the trailing line feed with LineFeed=false.
type DataflowLogSinkOptions struct {
	Format      DataflowLogFormat
	Layout      string
	Title       string
	LineFeed    bool
	LineFeedSet bool
	Writer      DataflowLogWriter
}

func (o DataflowLogSinkOptions) normalized() (DataflowLogSinkOptions, error) {
	result := o
	format := strings.ToLower(strings.TrimSpace(string(result.Format)))
	if format == "" {
		format = string(DataflowLogSummary)
	}
	switch DataflowLogFormat(format) {
	case DataflowLogSummary, DataflowLogJSON, DataflowLogXML:
	default:
		return DataflowLogSinkOptions{}, NewError(ErrorInvalidRule, fmt.Sprintf("unsupported dataflow log format %q", result.Format))
	}
	result.Format = DataflowLogFormat(format)
	result.Title = strings.TrimSpace(result.Title)
	if !result.LineFeedSet {
		result.LineFeed = true
	}
	return result, nil
}

func (o DataflowLogSinkOptions) validate() error {
	_, err := o.normalized()
	return err
}

func defaultDataflowLogWriter(ctx context.Context, line string) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	_, err := fmt.Print(line)
	return err
}

func dataflowLogPortIndex(operator DataflowOperator, inputPort string) int {
	for index, port := range operator.InputPorts {
		if port == inputPort {
			return index
		}
	}
	return 0
}

func (d *DataflowInstance) writeDataflowLog(ctx context.Context, operator DataflowOperator, inputPort string, value any) error {
	if operator.LogOptions == nil {
		return nil
	}
	options, err := operator.LogOptions.normalized()
	if err != nil {
		return err
	}
	rendered, err := renderDataflowLogValue(options.Format, value)
	if err != nil {
		return err
	}
	port := dataflowLogPortIndex(operator, inputPort)
	if options.Layout == "" {
		var builder strings.Builder
		fmt.Fprintf(&builder, "[%s] ", d.definition.name)
		if options.Title != "" {
			fmt.Fprintf(&builder, "[%s] ", options.Title)
		}
		if d.options.InstanceID != "" {
			fmt.Fprintf(&builder, "[%s] ", d.options.InstanceID)
		}
		fmt.Fprintf(&builder, "[port %d] %s", port, rendered)
		rendered = builder.String()
	} else {
		rendered = strings.NewReplacer(
			"%df", d.definition.name,
			"%p", fmt.Sprintf("%d", port),
			"%i", d.options.InstanceID,
			"%t", options.Title,
			"%e", rendered,
		).Replace(options.Layout)
	}
	if options.LineFeed {
		rendered += "\n"
	} else {
		rendered = strings.ReplaceAll(rendered, "\r", "")
		rendered = strings.ReplaceAll(rendered, "\n", "")
	}
	writer := options.Writer
	if writer == nil {
		writer = defaultDataflowLogWriter
	}
	return writer(ctx, rendered)
}

func renderDataflowLogValue(format DataflowLogFormat, value any) (string, error) {
	switch typed := value.(type) {
	case Event:
		switch format {
		case DataflowLogJSON:
			return RenderJSON(typed, WithJSONTitle(typed.TypeName()))
		case DataflowLogXML:
			return RenderXML(typed)
		default:
			return fmt.Sprintf("%s[%v]", typed.TypeName(), typed.Underlying()), nil
		}
	case Row:
		switch format {
		case DataflowLogJSON:
			return renderDataflowLogRowJSON(typed)
		case DataflowLogXML:
			return renderDataflowLogRowXML(typed)
		default:
			return fmt.Sprintf("%s[%v]", typed.Schema().Name(), typed.AsMap()), nil
		}
	default:
		switch format {
		case DataflowLogJSON:
			encoded, err := json.Marshal(value)
			if err != nil {
				return "", fmt.Errorf("esper: render dataflow log JSON: %w", err)
			}
			return string(encoded), nil
		case DataflowLogXML:
			encoded, err := xml.Marshal(value)
			if err != nil {
				return "", fmt.Errorf("esper: render dataflow log XML: %w", err)
			}
			return string(encoded), nil
		default:
			return fmt.Sprint(value), nil
		}
	}
}

func renderDataflowLogRowJSON(row Row) (string, error) {
	payload, err := renderJSONObject(row.Schema().Fields(), row.Get, nil, 64, 0)
	if err != nil {
		return "", fmt.Errorf("esper: render dataflow log row JSON: %w", err)
	}
	encoded, err := json.Marshal(map[string]any{row.Schema().Name(): payload})
	if err != nil {
		return "", fmt.Errorf("esper: render dataflow log row JSON: %w", err)
	}
	return string(encoded), nil
}

func renderDataflowLogRowXML(row Row) (string, error) {
	config := XMLRenderConfig{Indent: "  ", MaxDepth: 64}
	node, err := xmlRenderNodeForObject(row.Schema().Name(), row.Schema().Fields(), row.Get, nil, config, 0)
	if err != nil {
		return "", fmt.Errorf("esper: render dataflow log row XML: %w", err)
	}
	var buffer bytes.Buffer
	buffer.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>")
	encoder := xml.NewEncoder(&buffer)
	encoder.Indent("", config.Indent)
	if err := writeXMLRenderNode(encoder, node); err != nil {
		return "", fmt.Errorf("esper: render dataflow log row XML: %w", err)
	}
	if err := encoder.Flush(); err != nil {
		return "", fmt.Errorf("esper: render dataflow log row XML: %w", err)
	}
	return buffer.String(), nil
}
