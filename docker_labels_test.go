package deploy

import (
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestDockerLabelExtractor_validate(t *testing.T) {
	type fields struct {
		Path string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "Test validate throws an error on empty path",
			fields: fields{
				Path: "",
			},
			wantErr: true,
		},
		{
			name: "Test validate throws an error on invalid path",
			fields: fields{
				Path: "testdata/Dockerfile.invalid-name",
			},
			wantErr: true,
		},
		{
			name: "Test validate returns nil if there are no errors",
			fields: fields{
				Path: "testdata/Dockerfile.public-api",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &DockerLabelExtractor{
				Path: tt.fields.Path,
			}
			if err := e.validate(); (err != nil) != tt.wantErr {
				t.Errorf("DockerLabelExtractor.validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDockerLabelExtractor_Extract(t *testing.T) {
	type fields struct {
		Path string
	}
	tests := []struct {
		name    string
		fields  fields
		want    map[string]string
		wantErr bool
	}{
		{
			name: "Test Extract fails validation on invalid Path",
			fields: fields{
				Path: "testdata/Dockerfile.invalid-name",
			},
			wantErr: true,
		},
		{
			name: "Test Extract returns no labels for an empty data file",
			fields: fields{
				Path: "testdata/Dockerfile.empty",
			},
			wantErr: false,
			want:    map[string]string{},
		},
		{
			name: "Test Extract returns no labels for a docker file with no labels",
			fields: fields{
				Path: "testdata/Dockerfile.no-labels",
			},
			wantErr: false,
			want:    map[string]string{},
		},
		{
			name: "Test Extract returns labels contained in the docker file",
			fields: fields{
				Path: "testdata/Dockerfile.public-api",
			},
			wantErr: false,
			want: map[string]string{
				"traefik.frontend.passHostHeader": "true",
				"traefik.frontend.entryPoints":    "http",
				"traefik.protocol":                "http",
				"traefik.backend":                 "api",
				"traefik.frontend.rule":           "PathPrefix:/v1/auth,/v1/admin,/v1/client,/v1/user,/v1/public,/health",
				"testfield":                       "foo=bar",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &DockerLabelExtractor{
				Path: tt.fields.Path,
			}
			got, err := e.Extract()
			if (err != nil) != tt.wantErr {
				t.Errorf("DockerLabelExtractor.Extract() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got, "Expected Labels were not returned")
		})
	}
}
