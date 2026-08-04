// Command readingdiff-seed compiles a patch with a hand-written edit plan and
// writes the result straight into the reading-diff cache.
//
// It exists so the viewer can be exercised against a real compiled abridgement
// without spending a model call, which makes UI work and browser testing fast
// and deterministic. The plan is supplied as JSON on the command line, so the
// same tool doubles as a way to reproduce a specific abridgement by hand when
// investigating one the generator produced.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/roasbeef/subtrate/internal/db"
	"github.com/roasbeef/subtrate/internal/readingdiff"
	"github.com/roasbeef/subtrate/internal/readingdiff/plangen"
	"github.com/roasbeef/subtrate/internal/store"
)

func main() {
	var (
		dbPath   = flag.String("db", "", "Path to the SQLite database")
		patchArg = flag.String("patch", "", "Path to the unified diff")
		planArg  = flag.String("plan", "", "Path to a JSON edit plan")
		model    = flag.String("model", plangen.DefaultModel,
			"Model id to record, which is part of the cache key")
	)
	flag.Parse()

	if *dbPath == "" || *patchArg == "" {
		log.Fatal("-db and -patch are required")
	}

	patch, err := os.ReadFile(*patchArg)
	if err != nil {
		log.Fatalf("read patch: %v", err)
	}

	plan := readingdiff.Plan{
		Remove:  []readingdiff.Range{},
		Fold:    []readingdiff.Range{},
		Replace: []readingdiff.Replacement{},
		Summary: "Seeded abridgement.",
	}
	if *planArg != "" {
		raw, err := os.ReadFile(*planArg)
		if err != nil {
			log.Fatalf("read plan: %v", err)
		}
		if err := json.Unmarshal(raw, &plan); err != nil {
			log.Fatalf("decode plan: %v", err)
		}
		if plan.Remove == nil {
			plan.Remove = []readingdiff.Range{}
		}
		if plan.Fold == nil {
			plan.Fold = []readingdiff.Range{}
		}
		if plan.Replace == nil {
			plan.Replace = []readingdiff.Replacement{}
		}
	}

	res, err := readingdiff.Compile(string(patch), plan)
	if err != nil {
		log.Fatalf("compile plan: %v", err)
	}

	dbStore, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() {
		if cerr := dbStore.Close(); cerr != nil {
			log.Printf("close database: %v", cerr)
		}
	}()

	cache := readingdiff.NewStoreCache(store.FromDB(dbStore.DB()))

	// The key must match what the service computes, or the seeded row is
	// simply never found.
	key := readingdiff.CacheKey(
		string(patch), *model, plangen.RubricHash(),
	)
	if err := cache.Store(
		context.Background(), key, *model, plangen.RubricHash(), res,
	); err != nil {
		log.Fatalf("store result: %v", err)
	}

	fmt.Printf("seeded %s\n", key[:16])
	fmt.Printf("  %s\n", res.ElisionLine())
	fmt.Printf("  segments: %d, folds: %d\n",
		len(res.Segments), res.Stats.FoldCount)
}
