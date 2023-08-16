package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessValueFunctions(t *testing.T) {
	testCases := []struct {
		name           string
		secrets        map[string]any
		parameters     map[string]any
		values         map[string]any
		expectedValues map[string]any
		err            string
	}{
		{
			name: "should inject values",
			secrets: map[string]any{
				"key": "secret",
			},
			parameters: map[string]any{
				"key": "parameter",
			},
			values: map[string]any{
				"key":  "value",
				"key2": "$parameters.key",
				"key3": "$secrets.key",
			},
			expectedValues: map[string]any{
				"key":  "value",
				"key2": "parameter",
				"key3": "secret",
			},
		},
		{
			name: "fails if the inject functions doesn't exist",
			values: map[string]any{
				"key":  "value",
				"key2": "$invalid.key",
			},
			err: "failed to inject value: unknown inject function: $invalid",
		},
		{
			name: "fails if inject key can't be found",
			values: map[string]any{
				"key":  "value",
				"key2": "$secrets.key",
			},
			err: "failed to inject value: secret with key key not found",
		},
		{
			name: "fails if inject value is of invalid format",
			values: map[string]any{
				"key":  "value",
				"key2": "$.key",
			},
			err: "failed to inject value: unknown inject function: $",
		},
		{
			name: "fails if inject value is missing from func key exp: `$secrets.`",
			values: map[string]any{
				"key":  "value",
				"key2": "$secrets.",
			},
			err: "failed to inject value: missing value from func key: $secrets.",
		},
		{
			name: "all in all with complex objects",
			secrets: map[string]any{
				"key": "secret",
			},
			parameters: map[string]any{
				"key": "parameter",
			},
			values: map[string]any{
				"key": "value",
				"key2": map[string]any{
					"key": []any{
						1,
						2,
						"$secrets.key",
						"3",
						map[string]any{
							"key":  "$secrets.key",
							"key2": "value",
						},
					},
				},
				"key3": map[string]any{
					"key": "$parameters.key",
				},
			},
			expectedValues: map[string]any{
				"key": "value",
				"key2": map[string]any{
					"key": []any{
						1,
						2,
						"secret",
						"3",
						map[string]any{
							"key":  "secret",
							"key2": "value",
						},
					},
				},
				"key3": map[string]any{
					"key": "parameter",
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()

			err := processValueFunctions(tc.secrets, tc.parameters, tc.values)
			if tc.err == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedValues, tc.values)
			} else {
				assert.EqualError(t, err, tc.err)
			}
		})
	}
}
