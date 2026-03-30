/*
Copyright © 2026 hoangkhoachau <hoangkhoachau@gmail.com>
*/
package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	insecurecreds "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/dynamicpb"

	"req2/internal/cache"
	compiler "req2/internal/proto"
	"req2/internal/utils"
)

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}

var cfgFile string

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
		protos, _ := cmd.PersistentFlags().GetStringSlice("proto")
		return completionCandidates(cmd.Context(), protos, toComplete), cobra.ShellCompDirectiveNoFileComp
	},
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		protos, _ := cmd.PersistentFlags().GetStringSlice("proto")
		method, ok := resolveMethod(cmd.Context(), protos, args[0])
		if !ok {
			fatal(fmt.Errorf("method not found: %s", args[0]))
		}

		address := viper.GetString("address")
		var creds credentials.TransportCredentials
		if viper.GetBool("insecure") {
			creds = insecurecreds.NewCredentials()
		} else {
			creds = credentials.NewTLS(&tls.Config{})
		}
		conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(creds))
		if err != nil {
			fatal(err)
		}
		defer conn.Close()

		inputStr, err := jsonMarshal.Marshal(compiler.NewMessage(method.Input()))
		if err != nil {
			fatal(err)
		}

		defaultInput := string(inputStr)
		input := defaultInput
		if cachedInput, err := cache.RetrieveCache(defaultInput); err == nil {
			input = cachedInput
		} else if !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "Warning: failed to retrieve cache:", err)
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
				fatal(err)
			}
			input = string(stdinBytes)
		case !repeat:
			input, err = utils.Edit(input)
			if err != nil {
				fatal(err)
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
					fatal(fmt.Errorf("invalid header (expected key:value): %s", h))
				}
				md.Append(strings.TrimSpace(key), strings.TrimSpace(value))
			}
			reqCtx = metadata.NewOutgoingContext(reqCtx, md)
		}

		req := dynamicpb.NewMessage(method.Input())
		if err := protojson.Unmarshal([]byte(input), req); err != nil {
			fatal(err)
		}
		rsp := dynamicpb.NewMessage(method.Output())
		if err := conn.Invoke(reqCtx, rpcPath(method), req, rsp); err != nil {
			fatal(err)
		}

		outputStr, err := jsonMarshal.Marshal(rsp)
		if err != nil {
			fatal(err)
		}

		fmt.Println(string(outputStr))

		if err = cache.StoreCache(defaultInput, input); err != nil {
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

func ensureConfig(configDir, configPath string) {
	if _, err := os.Stat(configPath); err == nil {
		return
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, "Error creating config dir:", err)
		return
	}
	defaultConfig := "address: \"\"\ninsecure: false\ntimeout: 30s\n"
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "Error creating config file:", err)
		return
	}
	fmt.Fprintln(os.Stderr, "Created default config file:", configPath)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		configDir := filepath.Join(home, ".config", "req2")
		ensureConfig(configDir, filepath.Join(configDir, "config.yaml"))
		viper.AddConfigPath(configDir)
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if cfgFile != "" {
			fatal(err)
		}
	}
}
