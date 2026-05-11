//go:build ignore

package main

import (
	"log"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
)

func main() {
	if err := entc.Generate("./schema", &gen.Config{
		Features: []gen.Feature{gen.FeaturePrivacy, gen.FeatureEntQL, gen.FeatureSnapshot},
	}); err != nil {
		log.Fatalf("running ent codegen: %v", err)
	}
}
