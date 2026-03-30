/*
Copyright © 2026 hoangkhoachau <hoangkhoachau@gmail.com>
*/
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"req2/internal/cache"
	grpcclient "req2/internal/grpc"
	compiler "req2/internal/proto"
	"req2/internal/utils"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var cfgFile string
var methods map[string]protoreflect.MethodDescriptor
var messages map[string]protoreflect.MessageDescriptor

var jsonMarshal = protojson.MarshalOptions{
	Multiline:       true,
	Indent:          "  ",
	EmitUnpopulated: true,
}

func methodFullName(m protoreflect.MethodDescriptor) string {
	return string(m.Parent().Name()) + "/" + string(m.Name())
}

func resolveMethod(cmd *cobra.Command, name string) protoreflect.MethodDescriptor {
	populateMethods(cmd)
	method, found := methods[name]
	if !found {
		fmt.Fprintln(os.Stderr, "Method not found:", name)
		os.Exit(1)
	}
	return method
}

func completionCandidates(cmd *cobra.Command, toComplete string) []string {
	populateMethods(cmd)
	var names []string
	for name, m := range methods {
		if strings.HasPrefix(name, toComplete) {
			desc := string(m.Input().Name()) + " → " + string(m.Output().Name())
			names = append(names, name+"\t"+desc)
		}
	}
	return names
}

func messageCompletionCandidates(cmd *cobra.Command, toComplete string) []string {
	populateMethods(cmd)
	var names []string
	for name := range messages {
		if strings.HasPrefix(name, toComplete) {
			names = append(names, name)
		}
	}
	return names
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "req2 <Service/Method>",
	Short: "A simple gRPC testing tool",
	Long: `req2 is a simple CLI tool for testing gRPC endpoints.

It compiles .proto files, generates a JSON request template for the selected
method, opens it in $EDITOR, sends the request, and caches the input for reuse.

Example:
  req2 -a localhost:50051 -p ./api.proto Greeter/SayHello`,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completionCandidates(cmd, toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		method := resolveMethod(cmd, args[0])

		address := viper.GetString("address")
		insecure := viper.GetBool("insecure")

		client, err := grpcclient.NewGrpcClient(address, !insecure)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		defer client.Close()

		inputStr, err := jsonMarshal.Marshal(compiler.NewMessage(method.Input()))
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		template := string(inputStr)
		input := template
		if cachedInput, err := cache.RetrieveCache(template); err == nil {
			input = cachedInput
		}

		repeat, _ := cmd.Flags().GetBool("repeat")
		fromStdin := false
		if stdinStat, err := os.Stdin.Stat(); err == nil {
			fromStdin = stdinStat.Mode()&os.ModeCharDevice == 0
		}

		switch {
		case fromStdin:
			stdinBytes, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			input = string(stdinBytes)
		case !repeat:
			input, err = utils.Edit(input)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
		}

		reqCtx := cmd.Context()
		if timeout := viper.GetDuration("timeout"); timeout > 0 {
			var cancel context.CancelFunc
			reqCtx, cancel = context.WithTimeout(reqCtx, timeout)
			defer cancel()
		}

		if headers, _ := cmd.Flags().GetStringArray("header"); len(headers) > 0 {
			md := metadata.New(nil)
			for _, h := range headers {
				key, value, ok := strings.Cut(h, ":")
				if !ok {
					fmt.Fprintln(os.Stderr, "Invalid header (expected key:value):", h)
					os.Exit(1)
				}
				md.Append(strings.TrimSpace(key), strings.TrimSpace(value))
			}
			reqCtx = metadata.NewOutgoingContext(reqCtx, md)
		}

		resp, err := client.SendRequest(reqCtx, input, method)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

		outputStr, err := jsonMarshal.Marshal(resp)
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
	rootCmd.Flags().DurationP("timeout", "t", 0, "request timeout (e.g. 30s, 1m); 0 means no timeout")
	rootCmd.Flags().StringArrayP("header", "H", nil, "gRPC metadata header (key:value), repeatable")

	viper.BindPFlag("address", rootCmd.Flags().Lookup("address"))
	viper.BindPFlag("insecure", rootCmd.Flags().Lookup("insecure"))
	viper.BindPFlag("timeout", rootCmd.Flags().Lookup("timeout"))
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
			defaultConfig := "address: \"\"\ninsecure: false\ntimeout: 30s\n"
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
	if methods != nil {
		return
	}
	methods = make(map[string]protoreflect.MethodDescriptor)
	messages = make(map[string]protoreflect.MessageDescriptor)
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

		for _, m := range compiler.ListMethods(compiler.ListServices(res)) {
			methods[methodFullName(m)] = m
			messages[string(m.Input().Name())] = m.Input()
			messages[string(m.Output().Name())] = m.Output()
		}
	}
}
