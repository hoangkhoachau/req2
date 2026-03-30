package cmd

import (
	"fmt"
	compiler "github.com/hoangkhoachau/req2/internal/proto"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect <Service/Method | MessageName>",
	Short: "Print input and output templates for a method or message",
	Args:  cobra.ExactArgs(1),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		protos, _ := cmd.Flags().GetStringSlice("proto")
		candidates := append(
			completionCandidates(cmd.Context(), protos, toComplete),
			messageCompletionCandidates(cmd.Context(), protos, toComplete)...,
		)
		return candidates, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		protos, _ := cmd.Flags().GetStringSlice("proto")
		r, err := buildRegistry(cmd.Context(), protos)
		if err != nil {
			return err
		}

		name := args[0]

		if method, ok := r.methods[name]; ok {
			return printMethod(method)
		}

		if msg, ok := r.messages[name]; ok {
			return printMessage(msg)
		}

		return fmt.Errorf("method or message not found: %s", name)
	},
	SilenceUsage: true,
}

func printMethod(method protoreflect.MethodDescriptor) error {
	input, err := jsonMarshal.Marshal(compiler.NewMessage(method.Input()))
	if err != nil {
		return err
	}
	output, err := jsonMarshal.Marshal(compiler.NewMessage(method.Output()))
	if err != nil {
		return err
	}
	fmt.Println("# Input")
	fmt.Println(string(input))
	fmt.Println("# Output")
	fmt.Println(string(output))
	return nil
}

func printMessage(msg protoreflect.MessageDescriptor) error {
	str, err := jsonMarshal.Marshal(compiler.NewMessage(msg))
	if err != nil {
		return err
	}
	fmt.Println(string(str))
	return nil
}

func init() {
	rootCmd.AddCommand(inspectCmd)
}
