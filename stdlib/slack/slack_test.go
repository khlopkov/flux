package slack_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
)

func TestSlack(t *testing.T) {
	ctx, deps := dependency.Inject(context.Background(), dependenciestest.Default())
	defer deps.Finish()

	_, scope, err := runtime.Eval(ctx, `
import "csv"
import "slack"

option url = "http://fakeurl.com/fakeyfake"
option token = "faketoken"

data = "
#datatype,string,string,string
#group,false,false,false
#default,_result,,
,result,qchannel,qtext,qcolor
,,fakeChannel,this is a lot of text yay,\"#FF0000\"
"

process = slack.endpoint(url:url, token:token)( mapFn:
	(r) => {
		return {channel:r.qchannel,text:r.qtext,color:r.color}
	}
)

csv.from(csv:data) |> process()

`)

	if err != nil {
		t.Error(err)
	}
	_ = scope
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
		sr := Request{
			URL:           r.URL.String(), // r.URL.String(),
			Authorization: r.Header.Get("Authorization"),
		}
		b, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			t.Error(err)
		}
		err = json.Unmarshal(b, &sr.PostData)
		if err != nil {
			t.Error(err)
		}
		err = json.Unmarshal(b, &sr.RawData)
		if err != nil {
			t.Error(err)
		}
		s.mu.Lock()
		s.requests = append(s.requests, sr)
		s.mu.Unlock()
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
func (s *Server) Close() {
	if s.closed {
		return
	}
	s.closed = true
	s.ts.Close()
}

type Request struct {
	URL           string
	Authorization string
	PostData      PostData

	// As part of fixing #3995, we need to ensure that the `as_user` key is
	// _absent_ from the JSON we send; it being present but null would still
	// result in Slack's API rejecting the request. Unfortunately, once the
	// request body has been Unmarshalled to a Struct, there is no way to
	// distinguish between an absent key and a null one; so we need to
	// unmarshal it as map[string]interface{} to perform the test.
	RawData map[string]interface{}
}

type PostData struct {
	Channel     string       `json:"channel"`
	Attachments []Attachment `json:"attachments"`
	AsUser      bool         `json:"as_user"`
}

type Attachment struct {
	Color    string   `json:"color"`
	Text     string   `json:"text"`
	MrkdwnIn []string `json:"mrkdwn_in"`
}

func TestSlackPost(t *testing.T) {

	s := NewServer(t)
	defer s.Close()

	testCases := []struct {
		name    string
		color   string
		text    string
		channel string
		URL     string
		token   string
	}{
		{
			name:    "....",
			color:   `warning`,
			text:    "aaaaaaab",
			channel: "general",
			URL:     s.URL,
			token:   "faketoken",
		},
		{
			name:    "....",
			color:   `#ffffff`,
			text:    "qaaaaaaab",
			channel: "general",
			URL:     s.URL,
			token:   "faketoken",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {

			fluxString := `import "csv"
import "slack"

endpoint = slack.endpoint(url:url, token:token)(mapFn: (r) => {
	return {channel:r.qchannel,text:r.qtext,color:r.wcolor}
})

csv.from(csv:data) |> endpoint()`
			extern := `
url = "` + tc.URL + `"
token = "` + tc.token + `"
data = "
#datatype,string,string,string,string,string
#group,false,false,false,false,false
#default,_result,,,,
,result,,qchannel,qtext,wcolor
,,,` + strings.Join([]string{tc.channel, tc.text, tc.color}, ",") + `"`

			ctx := flux.NewDefaultDependencies().Inject(context.Background())

			extHdl, err := runtime.Default.Parse(ctx, extern)
			if err != nil {
				t.Fatal(err)
			}
			prog, err := lang.Compile(ctx, fluxString, runtime.Default, time.Now(), lang.WithExtern(extHdl))
			if err != nil {
				t.Fatal(err)
			}
			query, err := prog.Start(ctx, &memory.ResourceAllocator{})

			if err != nil {
				t.Fatal(err)
			}
			res := <-query.Results()
			_ = res
			var HasSent bool
			err = res.Tables().Do(func(table flux.Table) error {
				return table.Do(func(reader flux.ColReader) error {
					if reader == nil {
						return nil
					}
					for i, meta := range reader.Cols() {
						if meta.Label == "_sent" {
							HasSent = true
							if v := reader.Strings(i).Value(0); string(v) != "true" {
								t.Fatalf("expecting _sent=true but got _sent=%v", string(v))
							}
						}
					}
					return nil
				})
			})
			if !HasSent {
				t.Fatal("expected a _sent column but didnt get one")
			}
			if err != nil {
				t.Fatal(err)
			}

			query.Done()
			if err := query.Err(); err != nil {
				t.Error(err)
			}
			reqs := s.Requests()

			if len(reqs) < 1 {
				t.Fatal("received no requests")
			}
			req := reqs[len(reqs)-1]

			if req.Authorization != "Bearer "+tc.token {
				t.Errorf("token incorrect got %s, expected %s", req.Authorization, "Bearer "+tc.token)
			}
			if len(req.PostData.Attachments) != 1 {
				t.Fatalf("expected 1 attachment got %d", len(req.PostData.Attachments))
			}
			if req.PostData.Attachments[0].Text != tc.text {
				t.Errorf(" got %s, expected text of %s", req.PostData.Attachments[0].Text, tc.text)
			}
			if req.PostData.Channel != tc.channel {
				t.Errorf("got channel: %s, expected %s", req.PostData.Channel, tc.channel)
			}
			if len(req.PostData.Attachments[0].MrkdwnIn) != 0 && req.PostData.Attachments[0].MrkdwnIn[0] != "text" {
				t.Errorf("mrkdwn_in field incorrect, should be lenth 1 with a string text in a json array")
			}
			if req.PostData.Attachments[0].Color != tc.color {
				t.Errorf("got color %s, expected %s", req.PostData.Attachments[0].Color, tc.color)
			}

			// This test can be removed on 2022-03-09; see #3531
			if _, hasKey := req.RawData["as_user"]; hasKey {
				t.Errorf("posted JSON included as_user key, Slack will reject this message from modern app tokens")
			}
		})
	}

}
