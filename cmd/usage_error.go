package cmd

import "github.com/spf13/cobra"

// usageError は使い方の誤り。
// cobra は使い方の誤りを型付きの error にしないため、Execute が使い方を表示するかの判定に使う。
type usageError struct {
	cmd *cobra.Command
	err error
}

func (e *usageError) Error() string { return e.err.Error() }

func (e *usageError) Unwrap() error { return e.err }

func noArgs(c *cobra.Command, args []string) error {
	if err := cobra.NoArgs(c, args); err != nil {
		return &usageError{cmd: c, err: err}
	}
	return nil
}

func flagUsageError(c *cobra.Command, err error) error {
	return &usageError{cmd: c, err: err}
}
