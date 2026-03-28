/*
Copyright © 2026 hoangkhoachau <hoangkhoachau@gmail.com>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	compiler "req2/internal/proto"
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var cfgFile string
var methods []protoreflect.MethodDescriptor

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "req2",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		populateMethods(cmd)
		methodNames := lo.FilterMap(methods, func(method protoreflect.MethodDescriptor, _ int) (string, bool) {
			return string(method.Name()), method.Name().IsValid() && strings.HasPrefix(string(method.Name()), toComplete)
		})
		return methodNames, cobra.ShellCompDirectiveNoFileComp
	},
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		populateMethods(cmd)
		method, found := lo.Find(methods, func(method protoreflect.MethodDescriptor) bool {
			return string(method.Name()) == args[0]
		})
		if !found {
			fmt.Fprintln(os.Stderr, "Method not found:", args[0])
			os.Exit(1)
		}
		fmt.Println("Selected method:", method.FullName(), method.Input().Name(), method.Output().Name())
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/req2/req2.yaml)")
	rootCmd.PersistentFlags().StringP("proto", "p", ".", "proto path")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".req2" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".req2")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func populateMethods(cmd *cobra.Command) {
	if len(methods) != 0 {
		return
	}
	proto := cmd.Flag("proto").Value.String()
	if proto == "" || proto == "." {
		return
	}
	res, err := compiler.Compile(
		cmd.Context(),
		proto,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	services := compiler.ListServices(res)
	methods = compiler.ListMethods(services)
}
