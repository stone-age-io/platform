package main

import (
	"slices"
	"testing"

	"github.com/spf13/cobra"
)

// Each step of init is a child process, so a root flag that is not forwarded
// sends that step to a different database: `--dir /data/pb_data` dropped from
// migrate would import the schema into ./pb_data and leave the real one bare.
// --nats and --nats-config belong to serve; init consumes --nats-config itself.
func TestInitForwardsTheRootFlagsItWasGiven(t *testing.T) {
	root := &cobra.Command{Use: "stone-age"}
	root.PersistentFlags().String("dir", "./pb_data", "")
	root.PersistentFlags().String("config", "", "")
	root.PersistentFlags().Bool("dev", false, "")
	root.PersistentFlags().Bool("nats", false, "")
	root.PersistentFlags().String("nats-config", "./nats-config/nats.conf", "")

	var got []string
	initCmd := &cobra.Command{Use: "init", Run: func(cmd *cobra.Command, _ []string) {
		got = forwardedFlags(cmd)
	}}
	root.AddCommand(initCmd)

	root.SetArgs([]string{"init", "--dir", "/data/pb_data", "--config", "/etc/sa.yaml", "--nats", "--nats-config", "/data/nc/nats.conf"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	slices.Sort(got)
	want := []string{"--config=/etc/sa.yaml", "--dir=/data/pb_data"}
	if !slices.Equal(got, want) {
		t.Errorf("forwardedFlags = %q, want %q (set root flags only, minus serve's)", got, want)
	}
}

// A flag left at its default is not forwarded, so the child applies the same
// default itself rather than being handed a copy of it.
func TestInitForwardsNothingWhenNoRootFlagIsSet(t *testing.T) {
	root := &cobra.Command{Use: "stone-age"}
	root.PersistentFlags().String("dir", "./pb_data", "")

	got := []string{"unset"}
	initCmd := &cobra.Command{Use: "init", Run: func(cmd *cobra.Command, _ []string) {
		got = forwardedFlags(cmd)
	}}
	root.AddCommand(initCmd)

	root.SetArgs([]string{"init"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("forwardedFlags = %q, want none", got)
	}
}
