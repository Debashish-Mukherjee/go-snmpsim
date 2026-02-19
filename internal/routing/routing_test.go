package routing

import "testing"

func TestRouterPriorityAndMatching(t *testing.T) {
	router, err := NewRouter([]Rule{
		{
			Match:  Matchers{},
			Action: Action{DatasetPath: "default.snmprec"},
		},
		{
			Match:  Matchers{DstPort: 20000},
			Action: Action{DatasetPath: "endpoint.snmprec"},
		},
		{
			Match:  Matchers{Community: "private"},
			Action: Action{DatasetPath: "community.snmprec"},
		},
		{
			Match:  Matchers{Context: "ctxA"},
			Action: Action{DatasetPath: "context.snmprec"},
		},
		{
			Match:  Matchers{Context: "ctxA", EngineID: "8000000001020304"},
			Action: Action{DatasetPath: "engine-context.snmprec"},
		},
	})
	if err != nil {
		t.Fatalf("NewRouter failed: %v", err)
	}

	tests := []struct {
		name string
		key  RequestKey
		want string
	}{
		{
			name: "engine_context_has_highest_priority",
			key: RequestKey{
				Community: "private",
				Context:   "ctxA",
				EngineID:  "8000000001020304",
				DstPort:   20000,
			},
			want: "engine-context.snmprec",
		},
		{
			name: "context_over_community",
			key: RequestKey{
				Community: "private",
				Context:   "ctxA",
				EngineID:  "different",
				DstPort:   20000,
			},
			want: "context.snmprec",
		},
		{
			name: "community_over_endpoint",
			key: RequestKey{
				Community: "private",
				DstPort:   20000,
			},
			want: "community.snmprec",
		},
		{
			name: "endpoint_over_default",
			key: RequestKey{
				Community: "public",
				DstPort:   20000,
			},
			want: "endpoint.snmprec",
		},
		{
			name: "default_fallback",
			key: RequestKey{
				Community: "public",
				DstPort:   20001,
			},
			want: "default.snmprec",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := router.Select(tc.key)
			if got != tc.want {
				t.Fatalf("Select() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRouterValidation(t *testing.T) {
	_, err := NewRouter([]Rule{{Match: Matchers{Community: "public"}, Action: Action{}}})
	if err == nil {
		t.Fatal("expected NewRouter to fail when datasetPath is empty")
	}
}
