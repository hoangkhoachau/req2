/*
Copyright © 2026 hoangkhoachau <hoangkhoachau@gmail.com>
*/
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"req2/internal/cache"
	grpcclient "req2/internal/grpc"
	compiler "req2/internal/proto"
	"req2/internal/utils"
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var cfgFile string
var methods []protoreflect.MethodDescriptor

func methodFullName(m protoreflect.MethodDescriptor) string {
	return string(m.Parent().Name()) + "/" + string(m.Name())
}

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
			fullName := methodFullName(method)
			return fullName, method.Name().IsValid() && strings.HasPrefix(fullName, toComplete)
		})
		return methodNames, cobra.ShellCompDirectiveNoFileComp
	},
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		populateMethods(cmd)
		method, found := lo.Find(methods, func(method protoreflect.MethodDescriptor) bool {
			return methodFullName(method) == args[0]
		})
		if !found {
			fmt.Fprintln(os.Stderr, "Method not found:", args[0])
			os.Exit(1)
		}
		address := viper.GetString("address")
		insecure := viper.GetBool("insecure")

		client, err := grpcclient.NewGrpcClient(address, !insecure)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer client.Close()

		inputStr, err := protojson.MarshalOptions{
			Multiline:       true,
			Indent:          "  ",
			EmitUnpopulated: true,
		}.Marshal(compiler.NewMessage(method.Input()))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		template := string(inputStr)
		input := template
		if cachedInput, err := cache.RetrieveCache(template); err == nil {
			input = cachedInput
		}

		if repeat, _ := cmd.Flags().GetBool("repeat"); !repeat {
			input, err = utils.Edit(input)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
		}

		resp, err := client.SendRequest(cmd.Context(), input, method)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		outputStr, err := protojson.MarshalOptions{
			Multiline:       true,
			Indent:          "  ",
			EmitUnpopulated: true,
		}.Marshal(resp)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		fmt.Println(string(outputStr))

		if err = cache.StoreCache(template, input); err != nil {
			fmt.Fprintln(os.Stderr, "Warning: failed to cache input:", err)
		}
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

	rootCmd.PersistentFlags().StringSliceP("proto", "p", []string{"."}, "proto paths")
	rootCmd.RegisterFlagCompletionFunc("proto",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{"proto"}, cobra.ShellCompDirectiveFilterFileExt
		})

	rootCmd.Flags().StringP("address", "a", "", "gRPC server address")
	rootCmd.Flags().BoolP("insecure", "i", false, "use insecure connection")
	rootCmd.Flags().BoolP("repeat", "r", false, "repeat request without editor input")

	viper.BindPFlag("address", rootCmd.Flags().Lookup("address"))
	viper.BindPFlag("insecure", rootCmd.Flags().Lookup("insecure"))
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

		viper.AddConfigPath(filepath.Join(home, ".config", "req2"))
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv() // read in environment variables that match

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); ok {
			home, _ := os.UserHomeDir()
			configDir := filepath.Join(home, ".config", "req2")
			configPath := filepath.Join(configDir, "config.yaml")
			if mkErr := os.MkdirAll(configDir, 0755); mkErr != nil {
				fmt.Fprintln(os.Stderr, "Error creating config dir:", mkErr)
				return
			}
			defaultConfig := "address: \"\"\ninsecure: false\n"
			if writeErr := os.WriteFile(configPath, []byte(defaultConfig), 0644); writeErr != nil {
				fmt.Fprintln(os.Stderr, "Error creating config file:", writeErr)
				return
			}
			fmt.Fprintln(os.Stderr, "Created default config file:", configPath)
			if readErr := viper.ReadInConfig(); readErr != nil {
				fmt.Fprintln(os.Stderr, "Error reading created config file:", readErr)
			}
		}
	}
}

func populateMethods(cmd *cobra.Command) {
	if len(methods) != 0 {
		return
	}
	protos, _ := cmd.PersistentFlags().GetStringSlice("proto")

	importPaths := make([]string, 0, len(protos))
	for _, proto := range protos {
		if proto == "" {
			continue
		}
		stat, err := os.Stat(proto)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		if stat.IsDir() {
			importPaths = append(importPaths, proto)
		} else {
			importPaths = append(importPaths, filepath.Dir(proto))
		}
	}

	for _, proto := range protos {
		if proto == "" {
			continue
		}
		res, err := compiler.Compile(
			cmd.Context(),
			proto,
			importPaths,
		)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		services := compiler.ListServices(res)
		methods = append(methods, compiler.ListMethods(services)...)
	}
}
