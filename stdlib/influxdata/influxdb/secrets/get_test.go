package secrets_test

import (
	"context"
	"testing"

	"github.com/InfluxCommunity/flux/dependencies/dependenciestest"
	"github.com/InfluxCommunity/flux/dependencies/secret"
	"github.com/InfluxCommunity/flux/dependency"
	"github.com/InfluxCommunity/flux/mock"
	"github.com/InfluxCommunity/flux/stdlib/influxdata/influxdb/secrets"
	"github.com/InfluxCommunity/flux/values"
)

func TestGet(t *testing.T) {
	for _, tt := range []struct {
		name    string
		secrets secret.Service
		args    map[string]values.Value
		want    values.Value
		err     string
	}{
		{
			name: "valid secret",
			secrets: mock.SecretService{
				"mykey": "myvalue",
			},
			args: map[string]values.Value{
				"key": values.NewString("mykey"),
			},
			want: values.NewString("myvalue"),
		},
		{
			name: "missing secret",
			secrets: mock.SecretService{
				"mykey": "myvalue",
			},
			args: map[string]values.Value{
				"key": values.NewString("missingkey"),
			},
			err: "secret key \"missingkey\" not found",
		},
		{
			name: "no secret service",
			args: map[string]values.Value{
				"key": values.NewString("mykey"),
			},
			err: "cannot retrieve secret \"mykey\": secret service uninitialized in dependencies",
		},
		{
			name:    "missing argument",
			secrets: mock.SecretService{},
			args:    map[string]values.Value{},
			err:     "missing required keyword argument \"key\"",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			deps := dependenciestest.Default()
			deps.Deps.Deps.SecretService = tt.secrets
			ctx, span := dependency.Inject(context.Background(), deps)
			defer span.Finish()

			args := values.NewObjectWithValues(tt.args)
			got, err := secrets.Get(ctx, args)
			if err != nil {
				if tt.err != "" {
					if want, got := tt.err, err.Error(); want != got {
						t.Fatalf("unexpected error -want/+got:\n\t- %q\n\t+ %q", want, got)
					}
				} else {
					t.Fatalf("unexpected error: %s", err)
				}
				return
			}

			if want := tt.want; !got.Equal(want) {
				t.Fatalf("unexpected value -want/+got:\n\t- %v\n\t+ %v", want, got)
			}
		})
	}
}
