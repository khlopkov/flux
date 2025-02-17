package zenoss_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/InfluxCommunity/flux"
	_ "github.com/InfluxCommunity/flux/csv"
	"github.com/InfluxCommunity/flux/dependencies/dependenciestest"
	"github.com/InfluxCommunity/flux/dependency"
	_ "github.com/InfluxCommunity/flux/fluxinit/static"
	"github.com/InfluxCommunity/flux/lang"
	"github.com/InfluxCommunity/flux/memory"
	"github.com/InfluxCommunity/flux/runtime"
	"github.com/google/go-cmp/cmp"
)

func TestZenoss(t *testing.T) {
	ctx, deps := dependency.Inject(context.Background(), dependenciestest.Default())
	defer deps.Finish()

	_, _, err := runtime.Eval(ctx, `
import "csv"
import "contrib/bonitoo-io/zenoss"

option url = "https://tenant.zenoss.io:8080/zport/dmd/evconsole_router"
option username = "admin"
option password = "12345"

data = "
#group,false,false,false,false,false,false,false,false,false
#datatype,string,string,string,string,string,string,string,string,string
#default,_result,,,,,,,,
,result,table,node,metric_type,resource,metric_name,message_key,description,severity
,,0,10.1.1.1,CPU,CPU-1,usage_idle,Alert-#1001,CPU-1 too busy,Critical
"

process = zenoss.endpoint(url: url, username: username, password: password)(mapFn: (r) => ({
    summary: r.description,
    device: r.node,
    component: "CPU",
    severity: r.severity,
    eventClass: "/App",
    eventClassKey: "",
    message: "",
    collector: "localhost",
}))

csv.from(csv:data) |> process()
`)

	if err != nil {
		t.Error(err)
	}
}

func TestZenossPost(t *testing.T) {
	s := NewServer(t)
	defer s.Close()

	testCases := []struct {
		name        string
		fn          string
		addEvent    AddEvent
		authHeaders map[string]string
	}{
		{
			name: "alert with username/password",
			addEvent: AddEvent{
				Action: "EventsRouter",
				Method: "add_event",
				Data: []Event{
					{
						Summary:    "some alert",
						EventClass: "/App",
						Severity:   "Warning",
					},
				},
				Type: "rpc",
				TID:  1,
			},
			fn:          `zenoss.endpoint(url: url, username: "a", password: "1")`,
			authHeaders: map[string]string{"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("a:1"))},
		},
		{
			name: "alert with apiKey",
			addEvent: AddEvent{
				Action: "EventsRouter",
				Method: "add_event",
				Data: []Event{
					{
						Summary:    "some alert",
						EventClass: "/App",
						Severity:   "Warning",
					},
				},
				Type: "rpc",
				TID:  1,
			},
			fn:          `zenoss.endpoint(url: url, apiKey: "my-api-key")`,
			authHeaders: map[string]string{"z-api-key": "my-api-key"},
		},
		{
			name: "alert with all fields",
			addEvent: AddEvent{
				Action: "CustomRouter",
				Method: "new_event",
				Data: []Event{
					{
						Summary:    "CPU-2 is too busy",
						Device:     "10.1.2.3",
						Component:  "CPU",
						EventClass: "/App",
						Severity:   "Warning",
						Collector:  "localhost",
					},
				},
				Type: "doc",
				TID:  1,
			},
			fn:          `zenoss.endpoint(url: url, username: "b", password: "2", action: action, method: method, type: type, tid: tid)`,
			authHeaders: map[string]string{"Authorization": "Basic " + base64.StdEncoding.EncodeToString([]byte("b:2"))},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s.Reset()

			event := tc.addEvent.Data[0]
			fluxString := `import "csv"
import "contrib/bonitoo-io/zenoss"

url = "` + s.URL + `"
action = "` + tc.addEvent.Action + `"
method = "` + tc.addEvent.Method + `"
type = "` + tc.addEvent.Type + `"
tid = ` + strconv.Itoa(tc.addEvent.TID) + `

data = "
#group,false,false,false,false,false,false,false,false,false
#datatype,string,string,string,string,string,string,string,string,string
#default,_result,,,,,,,,
,result,table,node,metric_type,resource,metric_name,message_key,description,severity
,,0,10.1.1.1,CPU,CPU-1,usage_idle,Alert-#1001,CPU-1 too busy,Critical
,,0,` + strings.Join([]string{event.Device, event.Component, "CPU-2", "usage_user", "Alert-#1002", event.Summary, event.Severity}, ",") + `
"

endpoint = ` + tc.fn + `(mapFn: (r) => ({
    summary: r.description,
    device: r.node,
    component: r.metric_type,
    severity: r.severity,
    eventClass: "/App",
    eventClassKey: "",
    message: "",
    collector: "` + event.Collector + `",
}))

csv.from(csv:data) |> endpoint()`

			ctx := flux.NewDefaultDependencies().Inject(context.Background())
			prog, err := lang.Compile(ctx, fluxString, runtime.Default, time.Now())
			if err != nil {
				t.Fatal(err)
			}

			query, err := prog.Start(ctx, &memory.ResourceAllocator{})
			if err != nil {
				t.Fatal(err)
			}

			var res flux.Result
			timer := time.NewTimer(1 * time.Second)
			select {
			case res = <-query.Results():
				timer.Stop()
			case <-timer.C:
				t.Fatal("query timeout")
			}

			var hasSent bool
			err = res.Tables().Do(func(table flux.Table) error {
				return table.Do(func(reader flux.ColReader) error {
					for i, meta := range reader.Cols() {
						if meta.Label == "_sent" {
							hasSent = true
							if v := reader.Strings(i).Value(0); string(v) != "true" {
								t.Fatalf("expecting _sent=true but got _sent=%v", string(v))
							}
							break
						}
					}
					return nil
				})
			})

			if err != nil {
				t.Fatal(err)
			}

			if !hasSent {
				t.Fatal("expected _sent column but didn't get one")
			}

			query.Done()
			if err := query.Err(); err != nil {
				t.Error(err)
			}

			reqs := s.Requests()
			if len(reqs) != 2 {
				t.Fatalf("expected 2 requests, received %d", len(reqs))
			}
			req := reqs[len(reqs)-1]
			if diff := cmp.Diff(tc.addEvent, req.AddEvent); diff != "" {
				t.Fatal(diff)
			}
			if diff := cmp.Diff(tc.authHeaders, req.authHeaders); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

type Server struct {
	mu       sync.Mutex
	ts       *httptest.Server
	URL      string
	requests []Request
	closed   bool
}

func NewServer(t *testing.T) *Server {
	s := new(Server)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaders := make(map[string]string)
		for _, authHeader := range []string{"Authorization", "z-api-key"} {
			if value := r.Header.Get(authHeader); value != "" {
				authHeaders[authHeader] = value
			}
		}
		sr := Request{
			URL:         r.URL.String(),
			authHeaders: authHeaders,
		}
		dec := json.NewDecoder(r.Body)
		err := dec.Decode(&sr.AddEvent)
		if err != nil {
			t.Error(err)
		}
		s.mu.Lock()
		s.requests = append(s.requests, sr)
		s.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	s.ts = ts
	s.URL = ts.URL

	return s
}

func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests
}

func (s *Server) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = []Request{}
}

func (s *Server) Close() {
	if s.closed {
		return
	}
	s.closed = true
	s.ts.Close()
}

type Request struct {
	URL         string
	AddEvent    AddEvent
	authHeaders map[string]string
}

type Event struct {
	Summary       string `json:"summary"`
	Device        string `json:"device"`
	Component     string `json:"component"`
	Severity      string `json:"severity"`
	EventClass    string `json:"evclass"`
	EventClassKey string `json:"evclasskey"`
	Collector     string `json:"collector"`
	Message       string `json:"message"`
}

type AddEvent struct {
	Action string  `json:"action"`
	Method string  `json:"method"`
	Data   []Event `json:"data"`
	Type   string  `json:"type"`
	TID    int     `json:"tid"`
}
