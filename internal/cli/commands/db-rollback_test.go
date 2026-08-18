package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The flag was registered as `Int32P("steps", ...)` and read back as
// `GetInt("step")` — wrong name and wrong type. pflag returned "flag accessed
// but not defined", so `foundation db:rollback` aborted for every possible
// invocation and never reached the migrator.
func TestRollbackSteps(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    int
		wantErr string
	}{
		{name: "long flag", args: []string{"--steps", "3"}, want: 3},
		{name: "shorthand", args: []string{"-s", "2"}, want: 2},
		{name: "default", args: nil, want: 1},
		{name: "zero", args: []string{"--steps", "0"}, wantErr: "must be a positive integer"},
		{name: "negative", args: []string{"--steps", "-2"}, wantErr: "must be a positive integer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// A fresh command per case: cobra flags keep their parsed value.
			cmd := *DBRollback
			cmd.ResetFlags()
			cmd.Flags().IntP(rollbackStepsFlag, "s", 1, "")

			require.NoError(t, cmd.ParseFlags(tt.args))

			steps, err := rollbackSteps(&cmd)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, steps)
		})
	}
}

// Guards the registration itself: `rollbackSteps` must be able to read the flag
// as it is actually declared on the real command.
func TestDBRollbackStepsFlagIsReadable(t *testing.T) {
	require.NoError(t, DBRollback.ParseFlags([]string{"--steps", "4"}))

	steps, err := rollbackSteps(DBRollback)
	require.NoError(t, err)
	assert.Equal(t, 4, steps)
}
