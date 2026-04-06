package config

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-playground/mold/v4"
)

func configPostProcess(cfg *Config) error {
	tform := mold.New()
	tform.Register("fieldLoader", fieldLoader)
	tform.Register("trim", fieldTrim)
	return tform.Struct(context.Background(), cfg)
}

func fieldLoader(ctx context.Context, fl mold.FieldLevel) error {
	val := fl.Field().String()
	if path, ok := strings.CutPrefix(val, "file:"); ok {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}
		fl.Field().SetString(string(data))
	} else if envVar, ok := strings.CutPrefix(val, "env:"); ok {
		fl.Field().SetString(os.Getenv(envVar))
	} else if plain, ok := strings.CutPrefix(val, "plain:"); ok {
		fl.Field().SetString(plain)
	}
	return nil
}

func fieldTrim(ctx context.Context, fl mold.FieldLevel) error {
	fl.Field().SetString(strings.TrimSpace(fl.Field().String()))
	return nil
}
