package compiler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"github.com/samber/lo"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func Compile(ctx context.Context, path string) ([]protoreflect.FileDescriptor, error) {
	fds := make([]protoreflect.FileDescriptor, 0)

	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		subFiles, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, subFile := range subFiles {
			if subFile.IsDir() || filepath.Ext(subFile.Name()) != ".proto" {
				continue
			}
			subFds, err := Compile(ctx, path+"/"+subFile.Name())
			if err != nil {
				return nil, err
			}
			fds = append(fds, subFds...)
		}
	} else {
		if filepath.Ext(path) != ".proto" {
			return nil, fmt.Errorf("invalid proto file: %s", path)
		}
		compiler := protocompile.Compiler{
			Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
				ImportPaths: []string{filepath.Dir(path)},
			}),
			MaxParallelism: 0,
			SourceInfoMode: protocompile.SourceInfoExtraComments,
		}
		res, err := compiler.Compile(ctx, filepath.Base(path))
		if err != nil {
			return nil, err
		}
		fds = append(fds, lo.Map(res, func(fd linker.File, _ int) protoreflect.FileDescriptor {
			return fd
		})...)
	}

	return fds, err
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

func ListMethods(fds []protoreflect.ServiceDescriptor) []protoreflect.MethodDescriptor {
	var methods []protoreflect.MethodDescriptor
	for _, service := range fds {
		for i := 0; i < service.Methods().Len(); i++ {
			methods = append(methods, service.Methods().Get(i))
		}
	}
	return methods
}
