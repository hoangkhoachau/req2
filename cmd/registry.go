package cmd

import (
	"context"
	"os"
	"path/filepath"
	compiler "github.com/hoangkhoachau/req2/internal/proto"
	"strings"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type registry struct {
	methods  map[string]protoreflect.MethodDescriptor
	messages map[string]protoreflect.MessageDescriptor
}

var (
	reg     *registry
	regOnce sync.Once
)

var jsonMarshal = protojson.MarshalOptions{
	Multiline:       true,
	Indent:          "  ",
	EmitUnpopulated: true,
}

func methodFullName(m protoreflect.MethodDescriptor) string {
	return string(m.Parent().Name()) + "/" + string(m.Name())
}

func rpcPath(m protoreflect.MethodDescriptor) string {
	return "/" + string(m.Parent().FullName()) + "/" + string(m.Name())
}

func loadRegistry(ctx context.Context, protoPaths []string) *registry {
	regOnce.Do(func() {
		var err error
		reg, err = buildRegistry(ctx, protoPaths)
		if err != nil {
			fatal(err)
		}
	})
	return reg
}

func buildRegistry(ctx context.Context, protoPaths []string) (*registry, error) {
	r := &registry{
		methods:  make(map[string]protoreflect.MethodDescriptor),
		messages: make(map[string]protoreflect.MessageDescriptor),
	}

	importPaths := make([]string, 0, len(protoPaths))
	for _, p := range protoPaths {
		if p == "" {
			continue
		}
		stat, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if stat.IsDir() {
			importPaths = append(importPaths, p)
		} else {
			importPaths = append(importPaths, filepath.Dir(p))
		}
	}

	for _, p := range protoPaths {
		if p == "" {
			continue
		}
		res, err := compiler.Compile(ctx, p, importPaths)
		if err != nil {
			return nil, err
		}
		for _, m := range compiler.ListMethods(res) {
			r.methods[methodFullName(m)] = m
			r.messages[string(m.Input().Name())] = m.Input()
			r.messages[string(m.Output().Name())] = m.Output()
		}
	}

	return r, nil
}

func resolveMethod(ctx context.Context, protoPaths []string, name string) (protoreflect.MethodDescriptor, bool) {
	r := loadRegistry(ctx, protoPaths)
	method, found := r.methods[name]
	return method, found
}

func completionCandidates(ctx context.Context, protoPaths []string, toComplete string) []string {
	r := loadRegistry(ctx, protoPaths)
	var names []string
	for name, m := range r.methods {
		if strings.HasPrefix(name, toComplete) {
			desc := string(m.Input().Name()) + " → " + string(m.Output().Name())
			names = append(names, name+"\t"+desc)
		}
	}
	return names
}

func messageCompletionCandidates(ctx context.Context, protoPaths []string, toComplete string) []string {
	r := loadRegistry(ctx, protoPaths)
	var names []string
	for name := range r.messages {
		if strings.HasPrefix(name, toComplete) {
			names = append(names, name)
		}
	}
	return names
}
