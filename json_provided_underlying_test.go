package esper

import (
	"context"
	"encoding/json"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"
)

type providedJSONFriend struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type providedJSONUser struct {
	ID            string               `json:"_id"`
	Index         int                  `json:"index"`
	GUID          string               `json:"guid"`
	IsActive      bool                 `json:"isActive"`
	Balance       string               `json:"balance"`
	Picture       string               `json:"picture"`
	Age           int                  `json:"age"`
	EyeColor      string               `json:"eyeColor"`
	Name          string               `json:"name"`
	Gender        string               `json:"gender"`
	Company       string               `json:"company"`
	Email         string               `json:"email"`
	Phone         string               `json:"phone"`
	Address       string               `json:"address"`
	About         string               `json:"about"`
	Registered    string               `json:"registered"`
	Latitude      float64              `json:"latitude"`
	Longitude     float64              `json:"longitude"`
	Tags          []string             `json:"tags"`
	Friends       []providedJSONFriend `json:"friends"`
	Greeting      string               `json:"greeting"`
	FavoriteFruit string               `json:"favoriteFruit"`
}

type providedJSONUsers struct {
	Users []providedJSONUser `json:"users"`
}

type providedJSONPartner struct {
	ID    int64     `json:"id"`
	Name  string    `json:"name"`
	Since time.Time `json:"since"`
}

type providedJSONEyeColor string

type providedJSONClient struct {
	ID         int64                 `json:"_id"`
	Index      int                   `json:"index"`
	GUID       UUID                  `json:"guid"`
	IsActive   bool                  `json:"isActive"`
	Balance    big.Rat               `json:"balance"`
	Picture    string                `json:"picture"`
	Age        int                   `json:"age"`
	EyeColor   providedJSONEyeColor  `json:"eyeColor"`
	Name       string                `json:"name"`
	Gender     string                `json:"gender"`
	Company    string                `json:"company"`
	Emails     []string              `json:"emails"`
	Phones     []int64               `json:"phones"`
	Address    string                `json:"address"`
	About      string                `json:"about"`
	Registered DateOnly              `json:"registered"`
	Latitude   float64               `json:"latitude"`
	Longitude  float64               `json:"longitude"`
	Tags       []string              `json:"tags"`
	Partners   []providedJSONPartner `json:"partners"`
}

type providedJSONClients struct {
	Clients []providedJSONClient `json:"clients"`
}

type providedJSONPrimitive struct {
	PrimitiveInt int `json:"primitiveInt"`
}

type providedJSONPrimitiveSource struct {
	IntBoxed *int `esper:"intBoxed"`
}

type providedJSONPatternOne struct {
	ID string `json:"id"`
}

type providedJSONPatternTwo struct {
	ID  string `json:"id"`
	Val int    `json:"val"`
}

type providedJSONPatternOut struct {
	StartEvent Event   `json:"startEvent"`
	EndEvents  []Event `json:"endEvents"`
}

const providedUsersJSON = `{"users":[{"_id":"U1","index":7,"guid":"g1","isActive":true,"balance":"B1","picture":"p1","age":23,"eyeColor":"BLUE","name":"User One","gender":"F","company":"C","email":"u@example.test","phone":"100","address":"A","about":"About","registered":"2020-01-02","latitude":2.5,"longitude":110.5,"tags":["x","y"],"friends":[{"id":"F1","name":"Friend One"}],"greeting":"Hi","favoriteFruit":"Apple"},{"_id":"U2","index":8,"guid":"g2","isActive":false,"balance":"B2","picture":"p2","age":33,"eyeColor":"GREEN","name":"User Two","gender":"M","company":"D","email":"v@example.test","phone":"101","address":"B","about":"More","registered":"2021-02-03","latitude":16.5,"longitude":114.2,"tags":[],"friends":[],"greeting":"Hello","favoriteFruit":"Pear"}]}`

const providedClientsJSON = `{"clients":[{"_id":4063715686146184700,"index":1951037102,"guid":"b7dc7f66-4f6d-4f03-14d7-83da210dfba6","isActive":true,"balance":0.8509300187678505,"picture":"picture","age":9,"eyeColor":"BROWN","name":"Client","gender":"D","company":"Company","emails":["a@example.test","b@example.test"],"phones":[1206223281],"address":"Address","about":"About","registered":"1961-10-09","latitude":26.9,"longitude":74.2,"tags":[],"partners":[{"id":-4413101314901277000,"name":"Partner","since":"1974-11-01T07:58:27.373380998Z"}]}]}`

func TestJSONProvidedUnderlyingUsersRoundTripMatchesEsper(t *testing.T) {
	schema, err := NewJSONSchemaFor[providedJSONUsers]("ProvidedUsers", nil)
	if err != nil {
		t.Fatal(err)
	}
	event, err := ParseJSON(schema, []byte(providedUsersJSON), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	underlying, ok := event.Underlying().(providedJSONUsers)
	if !ok || len(underlying.Users) != 2 || underlying.Users[0].Friends[0].Name != "Friend One" {
		t.Fatalf("provided Users underlying = %#v (%T)", event.Underlying(), event.Underlying())
	}
	if event.Get("users[1].favoriteFruit").Any() != "Pear" {
		t.Fatalf("provided Users property access = %v", event.Get("users[1].favoriteFruit"))
	}
	rendered, err := RenderJSON(event)
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEquivalent(t, providedUsersJSON, rendered)
}

func TestJSONProvidedUnderlyingClientsRoundTripAndSendMatchesEsper(t *testing.T) {
	schema, err := NewJSONSchemaFor[providedJSONClients]("ProvidedClients", nil)
	if err != nil {
		t.Fatal(err)
	}
	event, err := ParseJSON(schema, []byte(providedClientsJSON), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	underlying, ok := event.Underlying().(providedJSONClients)
	if !ok || len(underlying.Clients) != 1 {
		t.Fatalf("provided Clients underlying = %#v (%T)", event.Underlying(), event.Underlying())
	}
	client := underlying.Clients[0]
	if client.GUID.String() != "b7dc7f66-4f6d-4f03-14d7-83da210dfba6" || client.Registered != DateOnly("1961-10-09") || client.EyeColor != "BROWN" {
		t.Fatalf("provided Clients scalar conversion = %#v", client)
	}
	if client.Balance.Cmp(new(big.Rat).SetFrac64(8509300187678505, 10000000000000000)) != 0 || client.Partners[0].Since.Location() != time.UTC {
		t.Fatalf("provided Clients exact values = %#v", client)
	}

	env := NewEnvironment()
	if _, err := RegisterJSONFor[providedJSONClients](env, "ProvidedClientsEngine", nil); err != nil {
		t.Fatal(err)
	}
	plan, err := env.Build(From[providedJSONClients](env, "ProvidedClientsEngine").Query(StatementName("provided-json-clients")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	deployment, err := engine.Deploy(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	defer deployment.Undeploy(context.Background())
	var received []Event
	if _, err := deployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				received = append(received, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendJSON(context.Background(), "ProvidedClientsEngine", []byte(providedClientsJSON)); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 {
		t.Fatalf("provided Clients sent events = %d", len(received))
	}
	if _, ok := received[0].Underlying().(providedJSONClients); !ok {
		t.Fatalf("provided Clients sent underlying = %T", received[0].Underlying())
	}
	assertJSONEquivalent(t, providedClientsJSON, mustRenderJSON(t, received[0]))
}

func TestJSONProvidedUnderlyingCreateSchemaKeepsTypedNestedValues(t *testing.T) {
	cases := []struct {
		name   string
		field  string
		typ    reflect.Type
		input  string
		assert func(t *testing.T, event Event)
	}{
		{name: "Users", field: "users", typ: reflect.TypeOf([]providedJSONUser{}), input: providedUsersJSON, assert: func(t *testing.T, event Event) {
			users, ok := event.Get("users").Any().([]providedJSONUser)
			if !ok || len(users) != 2 || users[0].Friends[0].ID != "F1" {
				t.Fatalf("created Users nested value = %#v (%T)", event.Get("users").Any(), event.Get("users").Any())
			}
		}},
		{name: "Clients", field: "clients", typ: reflect.TypeOf([]providedJSONClient{}), input: providedClientsJSON, assert: func(t *testing.T, event Event) {
			clients, ok := event.Get("clients").Any().([]providedJSONClient)
			if !ok || len(clients) != 1 || clients[0].Registered != DateOnly("1961-10-09") {
				t.Fatalf("created Clients nested value = %#v (%T)", event.Get("clients").Any(), event.Get("clients").Any())
			}
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			schema, err := NewJSONSchema("Created"+testCase.name, []FieldSpec{FieldDef(testCase.field, testCase.typ)})
			if err != nil {
				t.Fatal(err)
			}
			event, err := ParseJSON(schema, []byte(testCase.input), time.Unix(0, 0).UTC())
			if err != nil {
				t.Fatal(err)
			}
			testCase.assert(t, event)
			assertJSONEquivalent(t, testCase.input, mustRenderJSON(t, event))
		})
	}
}

func TestJSONProvidedUnderlyingRejectsUnsupportedSchemaShapes(t *testing.T) {
	if _, err := NewJSONSchemaFor[providedJSONClients]("DynamicProvided", nil, AllowDynamicFields()); err == nil || !strings.Contains(err.Error(), "dynamic") {
		t.Fatalf("dynamic provided JSON schema error = %v", err)
	}
	parent, err := NewJSONSchema("ProvidedParent", []FieldSpec{FieldDef("id", reflect.TypeOf(""))})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewJSONSchemaFor[providedJSONClients]("InheritedProvided", nil, WithSchemaParent(parent)); err == nil || !strings.Contains(err.Error(), "parents") {
		t.Fatalf("parent provided JSON schema error = %v", err)
	}
	if _, err := NewJSONSchemaFor[providedJSONUsers]("UnknownProvidedField", []FieldSpec{FieldDef("missing", reflect.TypeOf(""))}); err == nil || !strings.Contains(err.Error(), "not present") {
		t.Fatalf("unknown provided JSON field error = %v", err)
	}
	if _, err := NewJSONSchemaFor[providedJSONUsers]("MismatchedProvidedField", []FieldSpec{FieldDef("users", reflect.TypeOf(""))}); err == nil || !strings.Contains(err.Error(), "declares") {
		t.Fatalf("mismatched provided JSON field error = %v", err)
	}
}

func TestJSONProvidedUnderlyingPreservesPrimitiveDefaultOnNull(t *testing.T) {
	schema, err := NewJSONSchemaFor[providedJSONPrimitive]("PrimitiveDefault", nil, WithJSONDefaults(map[string]any{
		"primitiveInt": -1,
	}))
	if err != nil {
		t.Fatal(err)
	}
	for _, payload := range []string{`{}`, `{"primitiveInt":null}`} {
		event, parseErr := ParseJSON(schema, []byte(payload), time.Unix(0, 0).UTC())
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		underlying, ok := event.Underlying().(providedJSONPrimitive)
		if !ok || underlying.PrimitiveInt != -1 {
			t.Fatalf("primitive default for %s = %#v (%T), want -1", payload, event.Underlying(), event.Underlying())
		}
	}
}

func TestJSONProvidedUnderlyingPrimitiveDefaultSurvivesNullProjection(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterStruct[providedJSONPrimitiveSource](env, "ProvidedPrimitiveSource"); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterJSONFor[providedJSONPrimitive](env, "ProvidedPrimitiveTarget", nil, WithJSONDefaults(map[string]any{
		"primitiveInt": -1,
	})); err != nil {
		t.Fatal(err)
	}
	routePlan, err := env.Build(Select(
		From[providedJSONPrimitiveSource](env, "ProvidedPrimitiveSource"),
		Alias("primitiveInt", Cast[*int, int](Field[providedJSONPrimitiveSource, *int]("intBoxed"))),
	).InsertInto("ProvidedPrimitiveTarget", StatementName("provided-json-primitive-default-route")))
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(From[providedJSONPrimitive](env, "ProvidedPrimitiveTarget").Query(StatementName("provided-json-primitive-default-consumer")))
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env)
	if _, err := engine.Deploy(context.Background(), routePlan); err != nil {
		t.Fatal(err)
	}
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer consumerDeployment.Undeploy(context.Background())
	var received []Event
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				received = append(received, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendEvent(context.Background(), providedJSONPrimitiveSource{}); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 {
		t.Fatalf("primitive default projected events = %d", len(received))
	}
	underlying, ok := received[0].Underlying().(providedJSONPrimitive)
	if !ok || underlying.PrimitiveInt != -1 {
		t.Fatalf("primitive default projected underlying = %#v (%T), want -1", received[0].Underlying(), received[0].Underlying())
	}
}

func TestJSONProvidedUnderlyingPatternInsertMaterializesTypedArray(t *testing.T) {
	env := NewEnvironment()
	if _, err := RegisterJSONFor[providedJSONPatternOne](env, "EventOne", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterJSONFor[providedJSONPatternTwo](env, "EventTwo", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := RegisterJSONFor[providedJSONPatternOut](env, "EventOut", nil); err != nil {
		t.Fatal(err)
	}
	one := From[providedJSONPatternOne](env, "EventOne")
	two := From[providedJSONPatternTwo](env, "EventTwo")
	repeated := PatternFrom(two, "e", Equal[string](Field[providedJSONPatternTwo, string]("id"), TagField[string]("s", "id"))).
		MatchUntilExpr(nil, nil).Until(TimerInterval(two, 10*time.Second))
	patternQuery := PatternFrom(one, "s", Literal(true)).Then(repeated).Select(
		Alias("startEvent", ArrayAt[Event](TagEvents("s"), Literal[int64](0))),
		Alias("endEvents", TagEvents("e")),
	).InsertInto("EventOut", StatementName("provided-json-pattern-insert"))
	consumerQuery := From[providedJSONPatternOut](env, "EventOut").Query(StatementName("provided-json-pattern-consumer"))
	patternPlan, err := env.Build(patternQuery)
	if err != nil {
		t.Fatal(err)
	}
	consumerPlan, err := env.Build(consumerQuery)
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(env, WithStartTime(time.Unix(0, 0).UTC()))
	patternDeployment, err := engine.Deploy(context.Background(), patternPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer patternDeployment.Undeploy(context.Background())
	consumerDeployment, err := engine.Deploy(context.Background(), consumerPlan)
	if err != nil {
		t.Fatal(err)
	}
	defer consumerDeployment.Undeploy(context.Background())
	var received []providedJSONPatternOut
	var receivedEvents []Event
	if _, err := consumerDeployment.Statements()[0].Subscribe(func(_ context.Context, batch ResultBatch) error {
		for _, result := range batch.New {
			if event, ok := result.Event(); ok {
				underlying, ok := event.Underlying().(providedJSONPatternOut)
				if !ok {
					continue
				}
				received = append(received, underlying)
				receivedEvents = append(receivedEvents, event)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendJSON(context.Background(), "EventOne", []byte(`{"id":"G1"}`)); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendJSON(context.Background(), "EventTwo", []byte(`{"id":"G1","val":2}`)); err != nil {
		t.Fatal(err)
	}
	if err := engine.SendJSON(context.Background(), "EventTwo", []byte(`{"id":"G1","val":3}`)); err != nil {
		t.Fatal(err)
	}
	if err := engine.AdvanceTime(context.Background(), time.Unix(10, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if len(received) != 1 || len(received[0].EndEvents) != 2 || received[0].StartEvent.Get("id").Any() != "G1" || received[0].EndEvents[1].Get("val").Any() != 3 {
		t.Fatalf("pattern typed array output = %#v", received)
	}
	if len(receivedEvents) != 1 {
		t.Fatalf("pattern typed array events = %d", len(receivedEvents))
	}
	assertJSONEquivalent(t, `{"startEvent":{"id":"G1"},"endEvents":[{"id":"G1","val":2},{"id":"G1","val":3}]}`, mustRenderJSON(t, receivedEvents[0]))
}

func mustRenderJSON(t *testing.T, event Event) string {
	t.Helper()
	rendered, err := RenderJSON(event)
	if err != nil {
		t.Fatal(err)
	}
	return rendered
}

func assertJSONEquivalent(t *testing.T, expected, actual string) {
	t.Helper()
	decode := func(text string) any {
		decoder := json.NewDecoder(strings.NewReader(text))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			t.Fatalf("decode JSON %q: %v", text, err)
		}
		return value
	}
	if !reflect.DeepEqual(decode(expected), decode(actual)) {
		t.Fatalf("JSON values differ: expected=%s actual=%s", expected, actual)
	}
}
