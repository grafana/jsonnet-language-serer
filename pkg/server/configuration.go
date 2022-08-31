package server

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/formatter"
	"github.com/jdbaldry/go-language-server-protocol/jsonrpc2"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
)

type Configuration struct {
	ResolvePathsWithTanka bool
	JPaths                []string
	ExtVars               map[string]string
	FormattingOptions     formatter.Options

	EnableEvalDiagnostics bool
	EnableLintDiagnostics bool
}

func (s *server) DidChangeConfiguration(ctx context.Context, params *protocol.DidChangeConfigurationParams) error {
	settingsMap, ok := params.Settings.(map[string]interface{})
	if !ok {
		return fmt.Errorf("%w: unsupported settings payload. expected json object, got: %T", jsonrpc2.ErrInvalidParams, params.Settings)
	}

	for sk, sv := range settingsMap {
		switch sk {
		case "log_level":
			level, err := log.ParseLevel(sv.(string))
			if err != nil {
				return fmt.Errorf("%w: %v", jsonrpc2.ErrInvalidParams, err)
			}
			log.SetLevel(level)
		case "resolve_paths_with_tanka":
			s.configuration.ResolvePathsWithTanka = sv.(bool)
		case "jpath":
			svList := sv.([]interface{})
			s.configuration.JPaths = make([]string, len(svList))
			for i, v := range svList {
				s.configuration.JPaths[i] = v.(string)
			}
		case "enable_eval_diagnostics":
			s.configuration.EnableEvalDiagnostics = sv.(bool)
		case "enable_lint_diagnostics":
			s.configuration.EnableLintDiagnostics = sv.(bool)
		case "ext_vars":
			newVars, err := s.parseExtVars(sv)
			if err != nil {
				return fmt.Errorf("%w: ext_vars parsing failed: %v", jsonrpc2.ErrInvalidParams, err)
			}
			s.configuration.ExtVars = newVars
		case "formatting":
			newFmtOpts, err := s.parseFormattingOpts(sv)
			if err != nil {
				return fmt.Errorf("%w: formatting options parsing failed: %v", jsonrpc2.ErrInvalidParams, err)
			}
			s.configuration.FormattingOptions = newFmtOpts

		default:
			return fmt.Errorf("%w: unsupported settings key: %q", jsonrpc2.ErrInvalidParams, sk)
		}
	}
	log.Infof("configuration updated: %+v", s.configuration)

	return nil
}

func (s *server) parseExtVars(unparsed interface{}) (map[string]string, error) {
	newVars, ok := unparsed.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unsupported settings value for ext_vars. expected json object. got: %T", unparsed)
	}

	extVars := make(map[string]string, len(newVars))
	for varKey, varValue := range newVars {
		vv, ok := varValue.(string)
		if !ok {
			return nil, fmt.Errorf("unsupported settings value for ext_vars.%s. expected string. got: %T", varKey, varValue)
		}
		extVars[varKey] = vv
	}
	return extVars, nil
}

func (s *server) parseFormattingOpts(unparsed interface{}) (formatter.Options, error) {
	newOpts, ok := unparsed.(map[string]interface{})
	if !ok {
		return formatter.Options{}, fmt.Errorf("unsupported settings value for formatting. expected json object. got: %T", unparsed)
	}

	opts := formatter.DefaultOptions()
	config := mapstructure.DecoderConfig{
		Result: &opts,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			stringStyleDecodeFunc,
			commentStyleDecodeFunc,
		),
	}
	decoder, err := mapstructure.NewDecoder(&config)
	if err != nil {
		return formatter.Options{}, fmt.Errorf("decoder construction failed: %v", err)
	}

	if err := decoder.Decode(newOpts); err != nil {
		return formatter.Options{}, fmt.Errorf("map decode failed: %v", err)
	}
	return opts, nil
}

func resetExtVars(vm *jsonnet.VM, vars map[string]string) {
	vm.ExtReset()
	for vk, vv := range vars {
		vm.ExtVar(vk, vv)
	}
}

func stringStyleDecodeFunc(from, to reflect.Type, unparsed interface{}) (interface{}, error) {
	if to != reflect.TypeOf(formatter.StringStyleDouble) {
		return unparsed, nil
	}
	if from.Kind() != reflect.String {
		return nil, fmt.Errorf("expected string, got: %v", from.Kind())
	}

	// will not panic because of the kind == string check above
	switch str := unparsed.(string); str {
	case "double":
		return formatter.StringStyleDouble, nil
	case "single":
		return formatter.StringStyleSingle, nil
	case "leave":
		return formatter.StringStyleLeave, nil
	default:
		return nil, fmt.Errorf("expected one of 'double', 'single', 'leave', got: %q", str)
	}
}

func commentStyleDecodeFunc(from, to reflect.Type, unparsed interface{}) (interface{}, error) {
	if to != reflect.TypeOf(formatter.CommentStyleHash) {
		return unparsed, nil
	}
	if from.Kind() != reflect.String {
		return nil, fmt.Errorf("expected string, got: %v", from.Kind())
	}

	// will not panic because of the kind == string check above
	switch str := unparsed.(string); str {
	case "hash":
		return formatter.CommentStyleHash, nil
	case "slash":
		return formatter.CommentStyleSlash, nil
	case "leave":
		return formatter.CommentStyleLeave, nil
	default:
		return nil, fmt.Errorf("expected one of 'hash', 'slash', 'leave', got: %q", str)
	}
}
