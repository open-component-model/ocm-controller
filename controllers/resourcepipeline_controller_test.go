package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessValueFunctions(t *testing.T) {
	testCases := []struct {
		name           string
		parameters     map[string]any
		values         map[string]any
		expectedValues map[string]any
		err            string
	}{
		{
			name: "should inject values",
			parameters: map[string]any{
				"key": "parameter",
			},
			values: map[string]any{
				"key":  "value",
				"key2": "$parameters.key",
			},
			expectedValues: map[string]any{
				"key":  "value",
				"key2": "parameter",
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
				"key2": "$parameters.key",
			},
			err: "failed to inject value: parameter with key key not found",
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
				"key2": "$parameters.",
			},
			err: "failed to inject value: missing value from func key: $parameters.",
		},
		{
			name: "all in all with complex objects",
			parameters: map[string]any{
				"key": "parameter",
			},
			values: map[string]any{
				"key": "value",
				"key2": map[string]any{
					"key": []any{
						1,
						2,
						"$parameters.key",
						"3",
						map[string]any{
							"key":  "$parameters.key",
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
						"parameter",
						"3",
						map[string]any{
							"key":  "parameter",
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

			err := processValueFunctions(tc.parameters, tc.values)
			if tc.err == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedValues, tc.values)
			} else {
				assert.EqualError(t, err, tc.err)
			}
		})
	}
}
