package compiler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"slices"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"github.com/samber/lo"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

func Compile(ctx context.Context, path string, importPaths []string) ([]protoreflect.FileDescriptor, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	c := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: importPaths,
		}),
		MaxParallelism: 0,
		SourceInfoMode: protocompile.SourceInfoExtraComments,
	}

	var names []string
	if stat.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".proto" {
				names = append(names, entry.Name())
			}
		}
	} else {
		if filepath.Ext(path) != ".proto" {
			return nil, fmt.Errorf("invalid proto file: %s", path)
		}
		names = []string{filepath.Base(path)}
	}

	res, err := c.Compile(ctx, names...)
	if err != nil {
		return nil, err
	}
	return lo.Map(res, func(fd linker.File, _ int) protoreflect.FileDescriptor {
		return fd
	}), nil
}

func ListServices(fds []protoreflect.FileDescriptor) []protoreflect.ServiceDescriptor {
	var services []protoreflect.ServiceDescriptor
	for _, fd := range fds {
		for i := 0; i < fd.Services().Len(); i++ {
			services = append(services, fd.Services().Get(i))
		}
	}
	return services
}

func NewMessage(desc protoreflect.MessageDescriptor) *dynamicpb.Message {
	var fill func(protoreflect.MessageDescriptor, []protoreflect.FullName) *dynamicpb.Message
	fill = func(desc protoreflect.MessageDescriptor, path []protoreflect.FullName) *dynamicpb.Message {
		msg := dynamicpb.NewMessage(desc)
		for i := 0; i < desc.Fields().Len(); i++ {
			f := desc.Fields().Get(i)
			if f.Kind() != protoreflect.MessageKind || f.IsList() || f.IsMap() {
				continue
			}
			nested := f.Message()
			if !slices.Contains(path, nested.FullName()) {
				msg.Set(f, protoreflect.ValueOfMessage(fill(nested, append(path, desc.FullName()))))
			}
		}
		return msg
	}
	return fill(desc, nil)
}

func ListMethods(fds []protoreflect.ServiceDescriptor) []protoreflect.MethodDescriptor {
	var methods []protoreflect.MethodDescriptor
	for _, service := range fds {
		for i := 0; i < service.Methods().Len(); i++ {
			methods = append(methods, service.Methods().Get(i))
		}
	}
	return methods
}
