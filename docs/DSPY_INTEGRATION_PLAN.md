# DSPy Integration Plan for Substrate

> **Status**: Design Phase
> **Author**: Claude
> **Date**: 2026-02-01

## Executive Summary

This document outlines a comprehensive plan to integrate DSPy-inspired prompt engineering capabilities into Substrate. The integration will provide type-safe, composable prompt handling with automatic optimization, leveraging the existing actor system and Claude Agent SDK as the inference backend.

## Table of Contents

1. [Goals and Requirements](#goals-and-requirements)
2. [Architecture Overview](#architecture-overview)
3. [Core Components](#core-components)
4. [Implementation Phases](#implementation-phases)
5. [Detailed Design](#detailed-design)
6. [Database Schema](#database-schema)
7. [API Design](#api-design)
8. [Testing Strategy](#testing-strategy)
9. [Migration Path](#migration-path)

---

## Goals and Requirements

### Primary Goals

1. **Type-Safe Prompt Templates** - Strongly typed input/output signatures for prompts
2. **Composable Modules** - Chain-of-thought, retrieval-augmented, and multi-step reasoning
3. **Prompt Optimization** - Automatic prompt tuning via bootstrapping and teleprompters
4. **Actor-Based Execution** - Integrate with existing actor system for concurrent execution
5. **Claude SDK Backend** - Use Go Claude Agent SDK for all LLM inference

### Non-Goals

- Python interop (pure Go implementation)
- Direct DSPy API compatibility (inspired by, not a port of)
- Support for non-Claude models initially

### Key Requirements

| Requirement | Description | Priority |
|-------------|-------------|----------|
| Type Safety | Compile-time validation of prompt signatures | P0 |
| Composability | Modules can be nested and chained | P0 |
| Persistence | Store optimized prompts and traces | P0 |
| Observability | Full tracing of prompt execution | P1 |
| Optimization | Bootstrap few-shot examples from traces | P1 |
| Caching | Cache responses for identical inputs | P2 |

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Applications                                   │
│  (Agentic Review, Mail Summarization, Code Analysis, etc.)              │
└────────────────────────────────┬────────────────────────────────────────┘
                                 │
┌────────────────────────────────▼────────────────────────────────────────┐
│                         DSPy Actor Service                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │  Predict    │  │   ChainOf   │  │  ReAct      │  │  Program    │    │
│  │  Actor      │  │   Thought   │  │  Actor      │  │  Compose    │    │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘    │
│         │                │                │                │            │
│         └────────────────┴────────────────┴────────────────┘            │
│                                 │                                        │
│  ┌──────────────────────────────▼──────────────────────────────────┐    │
│  │                    Module Execution Engine                       │    │
│  │  - Signature validation                                          │    │
│  │  - Template rendering                                            │    │
│  │  - Output parsing                                                │    │
│  │  - Retry/fallback handling                                       │    │
│  └──────────────────────────────┬──────────────────────────────────┘    │
│                                 │                                        │
└─────────────────────────────────┼────────────────────────────────────────┘
                                  │
┌─────────────────────────────────▼────────────────────────────────────────┐
│                         Inference Layer                                   │
│  ┌──────────────────────────────────────────────────────────────────┐    │
│  │                    Claude Agent SDK Adapter                       │    │
│  │  - Spawner integration                                            │    │
│  │  - Session management                                             │    │
│  │  - Streaming support                                              │    │
│  │  - Token tracking                                                 │    │
│  └──────────────────────────────┬──────────────────────────────────┘    │
│                                 │                                        │
└─────────────────────────────────┼────────────────────────────────────────┘
                                  │
┌─────────────────────────────────▼────────────────────────────────────────┐
│                         Optimization Layer                                │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐  │
│  │  Trace Store    │  │  Bootstrapper   │  │  Teleprompter           │  │
│  │  (Examples DB)  │  │  (Few-shot Gen) │  │  (Prompt Optimizer)     │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────────────┘  │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
                                  │
┌─────────────────────────────────▼────────────────────────────────────────┐
│                         Storage Layer                                     │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐  │
│  │  Signatures     │  │  Traces         │  │  Optimized Prompts      │  │
│  │  (Templates)    │  │  (Executions)   │  │  (Compiled Programs)    │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────────────┘  │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## Core Components

### 1. Signatures (Type-Safe Templates)

Signatures define the contract for a prompt module - its inputs, outputs, and documentation.

```go
// internal/dspy/signature.go

// FieldType represents the type of a signature field.
type FieldType string

const (
    FieldTypeString   FieldType = "string"
    FieldTypeInt      FieldType = "int"
    FieldTypeBool     FieldType = "bool"
    FieldTypeList     FieldType = "list"
    FieldTypeJSON     FieldType = "json"
    FieldTypeMarkdown FieldType = "markdown"
)

// Field defines a single input or output field.
type Field struct {
    Name        string
    Type        FieldType
    Description string
    Required    bool
    Default     any
    Prefix      string  // Output parsing prefix (e.g., "Answer:")
}

// Signature defines the input/output contract for a module.
type Signature struct {
    Name        string
    Description string
    Inputs      []Field
    Outputs     []Field

    // Instructions are injected into the prompt.
    Instructions string

    // Examples are few-shot demonstrations.
    Examples []Example
}

// Example represents a single input/output demonstration.
type Example struct {
    Inputs  map[string]any
    Outputs map[string]any

    // Metadata for tracing.
    Source    string    // "manual", "bootstrapped", "optimized"
    Score     float64   // Quality score from evaluation
    CreatedAt time.Time
}
```

### 2. Modules (Composable Units)

Modules are the execution units that process inputs through a signature.

```go
// internal/dspy/module.go

// Module is the base interface for all DSPy modules.
type Module interface {
    // Forward executes the module with the given inputs.
    Forward(ctx context.Context, inputs map[string]any) fn.Result[map[string]any]

    // Signature returns the module's type signature.
    Signature() *Signature

    // Trace returns execution traces for optimization.
    Traces() []Trace
}

// Predict is the basic module that executes a single LLM call.
type Predict struct {
    sig      *Signature
    lm       LanguageModel
    traces   []Trace
    tracesMu sync.RWMutex
}

// ChainOfThought wraps a module with reasoning steps.
type ChainOfThought struct {
    inner Module

    // Extended signature with rationale field.
    extendedSig *Signature
}

// ReAct implements reasoning + acting with tool use.
type ReAct struct {
    inner   Module
    tools   []Tool
    maxIter int
}

// ProgramOfThought generates and executes code.
type ProgramOfThought struct {
    inner    Module
    executor CodeExecutor
}
```

### 3. Language Model Adapter

Bridges the DSPy module system to the Claude Agent SDK.

```go
// internal/dspy/lm.go

// LanguageModel abstracts the LLM backend.
type LanguageModel interface {
    // Generate produces a completion for the given prompt.
    Generate(ctx context.Context, prompt string, opts GenerateOpts) fn.Result[Generation]

    // GenerateN produces multiple completions.
    GenerateN(ctx context.Context, prompt string, n int, opts GenerateOpts) fn.Result[[]Generation]
}

// GenerateOpts configures generation behavior.
type GenerateOpts struct {
    Temperature   float64
    MaxTokens     int
    StopSequences []string
    SystemPrompt  string
}

// Generation represents a single LLM output.
type Generation struct {
    Text       string
    TokensIn   int
    TokensOut  int
    CostUSD    float64
    DurationMS int64
    Model      string
}

// ClaudeLM implements LanguageModel using the Claude Agent SDK.
type ClaudeLM struct {
    spawner *agent.Spawner
    config  *ClaudeLMConfig
}

// ClaudeLMConfig configures the Claude language model.
type ClaudeLMConfig struct {
    Model            string // e.g., "claude-opus-4-5-20251101"
    DefaultMaxTokens int
    DefaultTemp      float64
    Timeout          time.Duration

    // Session management for multi-turn.
    EnableSessions bool
    SessionTTL     time.Duration
}
```

### 4. Actor Integration

DSPy modules wrapped as actors for concurrent execution.

```go
// internal/dspy/actor.go

// DSPyRequest is the sealed union of all DSPy actor requests.
type DSPyRequest interface {
    actor.Message
    isDSPyRequest()
}

// PredictRequest executes a basic prediction.
type PredictRequest struct {
    actor.BaseMessage

    SignatureID string         // Reference to stored signature
    Inputs      map[string]any
    Options     *PredictOptions
}

func (PredictRequest) isDSPyRequest() {}

// PredictOptions configures prediction behavior.
type PredictOptions struct {
    Temperature   float64
    MaxTokens     int
    NumCompletions int
    UseCache      bool
}

// ChainOfThoughtRequest adds reasoning before prediction.
type ChainOfThoughtRequest struct {
    actor.BaseMessage

    SignatureID string
    Inputs      map[string]any
    RationaleHint string  // Optional hint for reasoning direction
}

func (ChainOfThoughtRequest) isDSPyRequest() {}

// OptimizeRequest triggers prompt optimization.
type OptimizeRequest struct {
    actor.BaseMessage

    SignatureID string
    TrainSet    []Example
    Metric      string      // Evaluation metric name
    Strategy    OptStrategy // bootstrap, mipro, etc.
}

func (OptimizeRequest) isDSPyRequest() {}

// DSPyResponse is the sealed union of all responses.
type DSPyResponse interface {
    isDSPyResponse()
}

// PredictResponse contains prediction results.
type PredictResponse struct {
    Outputs    map[string]any
    Rationale  string  // If ChainOfThought was used
    TraceID    string
    TokensUsed int
    CostUSD    float64
    Cached     bool
}

func (PredictResponse) isDSPyResponse() {}

// DSPyService implements the actor behavior.
type DSPyService struct {
    store     store.Storage
    lm        LanguageModel
    registry  *SignatureRegistry
    optimizer *Optimizer
    cache     *ResponseCache
}

// Receive dispatches requests to handlers.
func (s *DSPyService) Receive(ctx context.Context, msg DSPyRequest) fn.Result[DSPyResponse] {
    switch m := msg.(type) {
    case PredictRequest:
        return s.handlePredict(ctx, m)
    case ChainOfThoughtRequest:
        return s.handleChainOfThought(ctx, m)
    case OptimizeRequest:
        return s.handleOptimize(ctx, m)
    default:
        return fn.Err[DSPyResponse](fmt.Errorf("unknown request type: %T", msg))
    }
}
```

### 5. Optimization System

The heart of DSPy - automatic prompt optimization.

```go
// internal/dspy/optimize.go

// OptStrategy defines the optimization approach.
type OptStrategy string

const (
    // BootstrapFewShot generates examples from successful traces.
    BootstrapFewShot OptStrategy = "bootstrap_few_shot"

    // BootstrapFewShotWithRandomSearch adds hyperparameter tuning.
    BootstrapFewShotWithRandomSearch OptStrategy = "bootstrap_random"

    // MIPRO uses instruction generation + optimization.
    MIPRO OptStrategy = "mipro"

    // KNNFewShot uses semantic similarity for example selection.
    KNNFewShot OptStrategy = "knn_few_shot"
)

// Optimizer handles prompt optimization workflows.
type Optimizer struct {
    store      store.Storage
    lm         LanguageModel
    embedder   Embedder  // For KNN-based selection
    evaluator  Evaluator
}

// OptimizeConfig configures an optimization run.
type OptimizeConfig struct {
    Strategy      OptStrategy
    TrainSet      []Example
    ValSet        []Example
    Metric        MetricFunc
    MaxBootstraps int
    MaxRounds     int
    Temperature   float64
}

// MetricFunc evaluates a prediction against expected output.
type MetricFunc func(prediction, expected map[string]any) float64

// OptimizeResult contains optimization outcomes.
type OptimizeResult struct {
    OptimizedSignature *Signature
    BestExamples       []Example
    Scores             []float64
    TotalCost          float64
    Iterations         int
}

// Optimize runs the optimization loop.
func (o *Optimizer) Optimize(
    ctx context.Context,
    sig *Signature,
    cfg OptimizeConfig,
) fn.Result[*OptimizeResult] {
    switch cfg.Strategy {
    case BootstrapFewShot:
        return o.bootstrapFewShot(ctx, sig, cfg)
    case MIPRO:
        return o.mipro(ctx, sig, cfg)
    case KNNFewShot:
        return o.knnFewShot(ctx, sig, cfg)
    default:
        return fn.Err[*OptimizeResult](fmt.Errorf("unknown strategy: %s", cfg.Strategy))
    }
}
```

### 6. Trace Storage

Persistent storage for execution traces enabling optimization.

```go
// internal/dspy/trace.go

// Trace captures a single execution for analysis.
type Trace struct {
    ID          string
    SignatureID string
    ModuleType  string  // "predict", "chain_of_thought", "react"

    // Input/output snapshots.
    Inputs  map[string]any
    Outputs map[string]any

    // Intermediate steps (for CoT, ReAct).
    Steps []TraceStep

    // Metrics.
    Success    bool
    Score      float64
    TokensIn   int
    TokensOut  int
    CostUSD    float64
    DurationMS int64

    // Metadata.
    Model     string
    CreatedAt time.Time
    Tags      []string
}

// TraceStep captures an intermediate step.
type TraceStep struct {
    Type       string  // "thought", "action", "observation"
    Content    string
    TokensUsed int
    Timestamp  time.Time
}

// TraceStore persists and queries traces.
type TraceStore interface {
    // SaveTrace persists a trace.
    SaveTrace(ctx context.Context, trace *Trace) error

    // GetTracesBySignature retrieves traces for a signature.
    GetTracesBySignature(ctx context.Context, sigID string, opts TraceQueryOpts) ([]Trace, error)

    // GetSuccessfulExamples returns high-quality examples for bootstrapping.
    GetSuccessfulExamples(ctx context.Context, sigID string, limit int, minScore float64) ([]Example, error)

    // GetSimilarExamples finds semantically similar traces (for KNN).
    GetSimilarExamples(ctx context.Context, sigID string, inputs map[string]any, k int) ([]Example, error)
}
```

---

## Implementation Phases

### Phase 1: Foundation (Week 1-2)

**Goal**: Core DSPy primitives and basic prediction.

| Task | Description | Deliverable |
|------|-------------|-------------|
| 1.1 | Define signature types | `internal/dspy/signature.go` |
| 1.2 | Implement field validation | `internal/dspy/validate.go` |
| 1.3 | Create template renderer | `internal/dspy/template.go` |
| 1.4 | Build output parser | `internal/dspy/parser.go` |
| 1.5 | Implement Claude LM adapter | `internal/dspy/lm_claude.go` |
| 1.6 | Create basic Predict module | `internal/dspy/predict.go` |
| 1.7 | Add database migrations | `internal/db/migrations/` |
| 1.8 | Unit tests for all components | `internal/dspy/*_test.go` |

**Database Tables**:
- `dspy_signatures` - Signature definitions
- `dspy_examples` - Few-shot examples
- `dspy_traces` - Execution traces

### Phase 2: Actor Integration (Week 3)

**Goal**: Integrate DSPy with the actor system.

| Task | Description | Deliverable |
|------|-------------|-------------|
| 2.1 | Define actor message types | `internal/dspy/messages.go` |
| 2.2 | Implement DSPyService | `internal/dspy/service.go` |
| 2.3 | Create DSPy actor factory | `internal/dspy/actor.go` |
| 2.4 | Add to MCP tool registry | `internal/mcp/tools_dspy.go` |
| 2.5 | Integration tests | `tests/integration/dspy/` |

### Phase 3: Advanced Modules (Week 4)

**Goal**: Chain-of-thought and multi-step reasoning.

| Task | Description | Deliverable |
|------|-------------|-------------|
| 3.1 | Implement ChainOfThought | `internal/dspy/chain_of_thought.go` |
| 3.2 | Implement ReAct | `internal/dspy/react.go` |
| 3.3 | Implement ProgramOfThought | `internal/dspy/program_of_thought.go` |
| 3.4 | Add module composition | `internal/dspy/compose.go` |
| 3.5 | Tests for advanced modules | `internal/dspy/*_test.go` |

### Phase 4: Optimization (Week 5-6)

**Goal**: Prompt optimization via bootstrapping.

| Task | Description | Deliverable |
|------|-------------|-------------|
| 4.1 | Implement trace storage | `internal/dspy/trace_store.go` |
| 4.2 | Build evaluator framework | `internal/dspy/evaluate.go` |
| 4.3 | Implement BootstrapFewShot | `internal/dspy/bootstrap.go` |
| 4.4 | Implement MIPRO optimizer | `internal/dspy/mipro.go` |
| 4.5 | Add embedding support | `internal/dspy/embed.go` |
| 4.6 | Implement KNNFewShot | `internal/dspy/knn.go` |
| 4.7 | Optimization tests | `internal/dspy/optimize_test.go` |

### Phase 5: Applications (Week 7-8)

**Goal**: Build real applications using DSPy.

| Task | Description | Deliverable |
|------|-------------|-------------|
| 5.1 | Agentic Review module | `internal/apps/review/` |
| 5.2 | Mail summarization | `internal/apps/summarize/` |
| 5.3 | Code analysis | `internal/apps/codeqa/` |
| 5.4 | CLI integration | `cmd/substrate/dspy.go` |
| 5.5 | Web UI for signatures | `internal/web/templates/dspy/` |

---

## Detailed Design

### Template Rendering

Templates convert signatures + inputs into prompts.

```go
// internal/dspy/template.go

// TemplateRenderer generates prompts from signatures.
type TemplateRenderer struct {
    // Templates can be customized per signature.
    customTemplates map[string]*template.Template

    // Default template for standard signatures.
    defaultTemplate *template.Template
}

// Render produces a prompt string from signature and inputs.
func (r *TemplateRenderer) Render(sig *Signature, inputs map[string]any) (string, error) {
    data := TemplateData{
        Name:         sig.Name,
        Description:  sig.Description,
        Instructions: sig.Instructions,
        InputFields:  sig.Inputs,
        OutputFields: sig.Outputs,
        InputValues:  inputs,
        Examples:     sig.Examples,
    }

    tmpl := r.getTemplate(sig.Name)
    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, data); err != nil {
        return "", err
    }
    return buf.String(), nil
}

// Default prompt template structure:
const defaultPromptTemplate = `{{.Description}}

{{if .Instructions}}Instructions: {{.Instructions}}{{end}}

{{range .Examples}}
---
{{range $field := $.InputFields}}{{$field.Name}}: {{index $.InputValues $field.Name}}
{{end}}
{{range $field := $.OutputFields}}{{$field.Prefix}} {{index .Outputs $field.Name}}
{{end}}
---
{{end}}

{{range $field := .InputFields}}{{$field.Name}}: {{index $.InputValues $field.Name}}
{{end}}

{{range $field := .OutputFields}}{{$field.Prefix}}{{end}}`
```

### Output Parsing

Extract structured outputs from LLM responses.

```go
// internal/dspy/parser.go

// OutputParser extracts structured data from completions.
type OutputParser struct {
    // Fallback strategies when primary parsing fails.
    fallbacks []ParsingStrategy
}

// ParsingStrategy defines how to extract outputs.
type ParsingStrategy interface {
    Parse(response string, fields []Field) (map[string]any, error)
}

// PrefixParser looks for field prefixes (e.g., "Answer: ...")
type PrefixParser struct{}

func (p *PrefixParser) Parse(response string, fields []Field) (map[string]any, error) {
    result := make(map[string]any)

    for _, field := range fields {
        prefix := field.Prefix
        if prefix == "" {
            prefix = field.Name + ":"
        }

        // Find the prefix and extract until next prefix or end.
        idx := strings.Index(response, prefix)
        if idx == -1 {
            if field.Required {
                return nil, fmt.Errorf("required field %q not found", field.Name)
            }
            continue
        }

        // Extract value between this prefix and next.
        value := extractFieldValue(response[idx+len(prefix):], fields)

        // Convert to appropriate type.
        typed, err := convertValue(value, field.Type)
        if err != nil {
            return nil, fmt.Errorf("field %q: %w", field.Name, err)
        }
        result[field.Name] = typed
    }

    return result, nil
}

// JSONParser extracts JSON from markdown code blocks.
type JSONParser struct{}

// RegexParser uses custom regex patterns per field.
type RegexParser struct {
    Patterns map[string]*regexp.Regexp
}
```

### Response Caching

Cache identical requests to reduce costs.

```go
// internal/dspy/cache.go

// ResponseCache caches LLM responses by input hash.
type ResponseCache struct {
    store store.Storage
    ttl   time.Duration
}

// CacheKey generates a deterministic key from signature + inputs.
func (c *ResponseCache) CacheKey(sigID string, inputs map[string]any, opts GenerateOpts) string {
    h := sha256.New()
    h.Write([]byte(sigID))

    // Sort keys for deterministic ordering.
    keys := maps.Keys(inputs)
    sort.Strings(keys)
    for _, k := range keys {
        h.Write([]byte(k))
        h.Write([]byte(fmt.Sprintf("%v", inputs[k])))
    }

    // Include relevant options.
    h.Write([]byte(fmt.Sprintf("%.2f", opts.Temperature)))
    h.Write([]byte(fmt.Sprintf("%d", opts.MaxTokens)))

    return hex.EncodeToString(h.Sum(nil))
}

// Get retrieves a cached response.
func (c *ResponseCache) Get(ctx context.Context, key string) (*Generation, bool) {
    // Query from database...
}

// Set stores a response in cache.
func (c *ResponseCache) Set(ctx context.Context, key string, gen *Generation) error {
    // Store in database with TTL...
}
```

### Bootstrap Optimization Algorithm

```go
// internal/dspy/bootstrap.go

// bootstrapFewShot implements the core DSPy optimization loop.
func (o *Optimizer) bootstrapFewShot(
    ctx context.Context,
    sig *Signature,
    cfg OptimizeConfig,
) fn.Result[*OptimizeResult] {

    // 1. Generate initial traces by running on train set.
    traces := make([]Trace, 0, len(cfg.TrainSet))
    for _, example := range cfg.TrainSet {
        // Run prediction without few-shot examples.
        pred := NewPredict(sig, o.lm)
        result, err := pred.Forward(ctx, example.Inputs).Unpack()
        if err != nil {
            continue
        }

        // Evaluate against expected output.
        score := cfg.Metric(result, example.Outputs)

        trace := Trace{
            SignatureID: sig.Name,
            Inputs:      example.Inputs,
            Outputs:     result,
            Score:       score,
            Success:     score >= 0.5,
        }
        traces = append(traces, trace)
    }

    // 2. Select high-quality examples for bootstrapping.
    successfulTraces := filterSuccessful(traces, cfg.MaxBootstraps)

    // 3. Create optimized signature with examples.
    optimizedSig := sig.Clone()
    for _, trace := range successfulTraces {
        optimizedSig.Examples = append(optimizedSig.Examples, Example{
            Inputs:  trace.Inputs,
            Outputs: trace.Outputs,
            Source:  "bootstrapped",
            Score:   trace.Score,
        })
    }

    // 4. Evaluate on validation set.
    valScores := make([]float64, 0, len(cfg.ValSet))
    for _, example := range cfg.ValSet {
        pred := NewPredict(optimizedSig, o.lm)
        result, err := pred.Forward(ctx, example.Inputs).Unpack()
        if err != nil {
            valScores = append(valScores, 0)
            continue
        }
        score := cfg.Metric(result, example.Outputs)
        valScores = append(valScores, score)
    }

    return fn.Ok(&OptimizeResult{
        OptimizedSignature: optimizedSig,
        BestExamples:       optimizedSig.Examples,
        Scores:             valScores,
    })
}
```

---

## Database Schema

### Migrations

```sql
-- internal/db/migrations/000008_dspy_tables.up.sql

-- Signature definitions
CREATE TABLE dspy_signatures (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    instructions TEXT,
    input_schema TEXT NOT NULL,   -- JSON array of Field
    output_schema TEXT NOT NULL,  -- JSON array of Field
    version INTEGER DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX idx_signatures_name ON dspy_signatures(name);

-- Few-shot examples (can be manual or bootstrapped)
CREATE TABLE dspy_examples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    signature_id INTEGER NOT NULL REFERENCES dspy_signatures(id),
    inputs TEXT NOT NULL,        -- JSON object
    outputs TEXT NOT NULL,       -- JSON object
    source TEXT NOT NULL,        -- 'manual', 'bootstrapped', 'optimized'
    score REAL DEFAULT 0.0,
    active INTEGER DEFAULT 1,    -- Whether included in prompts
    created_at INTEGER NOT NULL,

    FOREIGN KEY (signature_id) REFERENCES dspy_signatures(id) ON DELETE CASCADE
);

CREATE INDEX idx_examples_signature ON dspy_examples(signature_id);
CREATE INDEX idx_examples_score ON dspy_examples(score DESC);

-- Execution traces for analysis and optimization
CREATE TABLE dspy_traces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    trace_id TEXT NOT NULL UNIQUE,
    signature_id INTEGER NOT NULL,
    module_type TEXT NOT NULL,
    inputs TEXT NOT NULL,         -- JSON object
    outputs TEXT,                 -- JSON object (null if failed)
    steps TEXT,                   -- JSON array of TraceStep
    success INTEGER NOT NULL,
    score REAL,
    tokens_in INTEGER,
    tokens_out INTEGER,
    cost_usd REAL,
    duration_ms INTEGER,
    model TEXT,
    created_at INTEGER NOT NULL,
    tags TEXT,                    -- JSON array of strings

    FOREIGN KEY (signature_id) REFERENCES dspy_signatures(id) ON DELETE CASCADE
);

CREATE INDEX idx_traces_signature ON dspy_traces(signature_id);
CREATE INDEX idx_traces_success ON dspy_traces(success);
CREATE INDEX idx_traces_score ON dspy_traces(score DESC);
CREATE INDEX idx_traces_created ON dspy_traces(created_at DESC);

-- Optimization runs
CREATE TABLE dspy_optimizations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    signature_id INTEGER NOT NULL,
    strategy TEXT NOT NULL,       -- 'bootstrap_few_shot', 'mipro', etc.
    config TEXT NOT NULL,         -- JSON OptimizeConfig
    result TEXT,                  -- JSON OptimizeResult
    status TEXT NOT NULL,         -- 'pending', 'running', 'completed', 'failed'
    train_size INTEGER,
    val_size INTEGER,
    best_score REAL,
    total_cost REAL,
    started_at INTEGER,
    completed_at INTEGER,
    created_at INTEGER NOT NULL,

    FOREIGN KEY (signature_id) REFERENCES dspy_signatures(id) ON DELETE CASCADE
);

CREATE INDEX idx_optimizations_signature ON dspy_optimizations(signature_id);
CREATE INDEX idx_optimizations_status ON dspy_optimizations(status);

-- Response cache
CREATE TABLE dspy_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cache_key TEXT NOT NULL UNIQUE,
    signature_id INTEGER NOT NULL,
    inputs_hash TEXT NOT NULL,
    response TEXT NOT NULL,       -- JSON Generation
    tokens_in INTEGER,
    tokens_out INTEGER,
    cost_usd REAL,
    hits INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,

    FOREIGN KEY (signature_id) REFERENCES dspy_signatures(id) ON DELETE CASCADE
);

CREATE INDEX idx_cache_key ON dspy_cache(cache_key);
CREATE INDEX idx_cache_expires ON dspy_cache(expires_at);
```

---

## API Design

### MCP Tools

```go
// internal/mcp/tools_dspy.go

// DSPy tools exposed via MCP.
func (s *Server) registerDSPyTools() {
    // Create or update a signature.
    mcp.AddTool(s.server, &mcp.Tool{
        Name: "dspy_create_signature",
        Description: "Define a new DSPy signature for type-safe prompting",
    }, s.handleCreateSignature)

    // Execute a prediction.
    mcp.AddTool(s.server, &mcp.Tool{
        Name: "dspy_predict",
        Description: "Run a prediction using a defined signature",
    }, s.handleDSPyPredict)

    // Chain of thought reasoning.
    mcp.AddTool(s.server, &mcp.Tool{
        Name: "dspy_chain_of_thought",
        Description: "Run prediction with step-by-step reasoning",
    }, s.handleChainOfThought)

    // Optimize a signature.
    mcp.AddTool(s.server, &mcp.Tool{
        Name: "dspy_optimize",
        Description: "Optimize a signature using bootstrapping",
    }, s.handleOptimize)

    // Add examples to a signature.
    mcp.AddTool(s.server, &mcp.Tool{
        Name: "dspy_add_example",
        Description: "Add a few-shot example to a signature",
    }, s.handleAddExample)

    // Query traces.
    mcp.AddTool(s.server, &mcp.Tool{
        Name: "dspy_list_traces",
        Description: "List execution traces for analysis",
    }, s.handleListTraces)
}
```

### CLI Commands

```bash
# Signature management
substrate dspy signature create --name "review" --input "code:string" --output "feedback:markdown"
substrate dspy signature list
substrate dspy signature show review

# Prediction
substrate dspy predict --signature review --input code="func foo() {}"
substrate dspy cot --signature review --input code="..."  # Chain of thought

# Examples
substrate dspy example add --signature review --input code="..." --output feedback="..."
substrate dspy example list --signature review

# Optimization
substrate dspy optimize --signature review --train-file train.json --strategy bootstrap
substrate dspy optimize status <optimization-id>

# Traces
substrate dspy trace list --signature review --limit 100
substrate dspy trace show <trace-id>
```

### gRPC Service

```protobuf
// internal/api/grpc/dspy.proto

service DSPy {
    // Signature management
    rpc CreateSignature(CreateSignatureRequest) returns (Signature);
    rpc GetSignature(GetSignatureRequest) returns (Signature);
    rpc ListSignatures(ListSignaturesRequest) returns (ListSignaturesResponse);

    // Prediction
    rpc Predict(PredictRequest) returns (PredictResponse);
    rpc ChainOfThought(ChainOfThoughtRequest) returns (ChainOfThoughtResponse);
    rpc StreamPredict(PredictRequest) returns (stream PredictChunk);

    // Optimization
    rpc StartOptimization(OptimizationRequest) returns (OptimizationStatus);
    rpc GetOptimizationStatus(OptimizationStatusRequest) returns (OptimizationStatus);

    // Traces
    rpc ListTraces(ListTracesRequest) returns (ListTracesResponse);
    rpc GetTrace(GetTraceRequest) returns (Trace);
}
```

---

## Testing Strategy

### Unit Tests

```go
// internal/dspy/predict_test.go

func TestPredict_Forward(t *testing.T) {
    t.Parallel()

    // Create mock LM.
    mockLM := &MockLanguageModel{
        response: "Answer: This is a test response.",
    }

    sig := &Signature{
        Name: "test_sig",
        Inputs: []Field{
            {Name: "question", Type: FieldTypeString, Required: true},
        },
        Outputs: []Field{
            {Name: "answer", Type: FieldTypeString, Prefix: "Answer:"},
        },
    }

    pred := NewPredict(sig, mockLM)

    result, err := pred.Forward(context.Background(), map[string]any{
        "question": "What is 2+2?",
    }).Unpack()

    require.NoError(t, err)
    require.Equal(t, "This is a test response.", result["answer"])
}

func TestChainOfThought_AddsRationale(t *testing.T) {
    t.Parallel()

    mockLM := &MockLanguageModel{
        response: `Reasoning: Let me think step by step...
Answer: 4`,
    }

    sig := &Signature{
        Name: "math",
        Inputs: []Field{
            {Name: "question", Type: FieldTypeString},
        },
        Outputs: []Field{
            {Name: "answer", Type: FieldTypeString, Prefix: "Answer:"},
        },
    }

    cot := NewChainOfThought(sig, mockLM)

    result, err := cot.Forward(context.Background(), map[string]any{
        "question": "What is 2+2?",
    }).Unpack()

    require.NoError(t, err)
    require.Equal(t, "4", result["answer"])
    require.Contains(t, result["rationale"], "step by step")
}
```

### Integration Tests

```go
// tests/integration/dspy/dspy_test.go

func TestDSPyIntegration_EndToEnd(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    store, cleanup := testDB(t)
    defer cleanup()

    // Create signature.
    sig := &dspy.Signature{
        Name:        "summarize",
        Description: "Summarize the given text concisely.",
        Inputs: []dspy.Field{
            {Name: "text", Type: dspy.FieldTypeString, Required: true},
        },
        Outputs: []dspy.Field{
            {Name: "summary", Type: dspy.FieldTypeString, Prefix: "Summary:"},
        },
    }

    // Create service with real Claude LM.
    spawner := agent.NewSpawner(agent.DefaultSpawnConfig())
    lm := dspy.NewClaudeLM(spawner, nil)
    svc := dspy.NewService(store, lm)

    // Run prediction.
    resp, err := svc.Predict(context.Background(), sig, map[string]any{
        "text": "The quick brown fox jumps over the lazy dog.",
    })

    require.NoError(t, err)
    require.NotEmpty(t, resp.Outputs["summary"])
}
```

### Property-Based Tests

```go
// internal/dspy/parser_test.go

func TestOutputParser_Properties(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        // Generate random field names and values.
        fieldName := rapid.StringMatching(`[a-z][a-z_]{2,10}`).Draw(t, "fieldName")
        fieldValue := rapid.String().Draw(t, "fieldValue")

        field := dspy.Field{
            Name:   fieldName,
            Type:   dspy.FieldTypeString,
            Prefix: fieldName + ":",
        }

        // Create response with the field.
        response := fmt.Sprintf("%s: %s", fieldName, fieldValue)

        // Parse should extract the value.
        parser := &dspy.PrefixParser{}
        result, err := parser.Parse(response, []dspy.Field{field})

        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }

        if result[fieldName] != strings.TrimSpace(fieldValue) {
            t.Fatalf("expected %q, got %q", fieldValue, result[fieldName])
        }
    })
}
```

---

## Migration Path

### Existing Code Integration

For extensions like Agentic Review that currently use raw prompts:

**Before (raw prompts)**:
```go
func (r *ReviewService) Review(ctx context.Context, code string) (string, error) {
    prompt := fmt.Sprintf(`Review this code and provide feedback:

%s

Provide specific, actionable feedback.`, code)

    resp, err := r.spawner.Spawn(ctx, prompt)
    return resp.Result, err
}
```

**After (DSPy integration)**:
```go
func (r *ReviewService) Review(ctx context.Context, code string) (*ReviewResult, error) {
    // Use pre-defined, optimized signature.
    result, err := r.dspy.Predict(ctx, "code_review", map[string]any{
        "code":     code,
        "language": detectLanguage(code),
    }).Unpack()
    if err != nil {
        return nil, err
    }

    return &ReviewResult{
        Feedback:    result["feedback"].(string),
        Severity:    result["severity"].(string),
        Suggestions: result["suggestions"].([]string),
    }, nil
}
```

### Gradual Adoption

1. **Phase 1**: Add DSPy alongside existing prompts (feature flag)
2. **Phase 2**: Migrate high-value prompts to signatures
3. **Phase 3**: Optimize signatures with production traces
4. **Phase 4**: Remove legacy prompt code

---

## Appendix A: DSPy Concepts Reference

| DSPy Concept | Go Implementation | Description |
|--------------|-------------------|-------------|
| Signature | `dspy.Signature` | Input/output specification |
| Module | `dspy.Module` interface | Executable prompt unit |
| Predict | `dspy.Predict` | Basic LLM call |
| ChainOfThought | `dspy.ChainOfThought` | Adds reasoning step |
| ReAct | `dspy.ReAct` | Reasoning + tool use |
| Teleprompter | `dspy.Optimizer` | Prompt optimization |
| BootstrapFewShot | `dspy.bootstrapFewShot()` | Example generation |
| Trace | `dspy.Trace` | Execution record |
| Example | `dspy.Example` | Few-shot demonstration |

## Appendix B: File Structure

```
internal/dspy/
├── signature.go       # Signature and Field types
├── validate.go        # Input validation
├── template.go        # Prompt template rendering
├── parser.go          # Output parsing strategies
├── module.go          # Module interface
├── predict.go         # Basic Predict module
├── chain_of_thought.go # CoT wrapper module
├── react.go           # ReAct implementation
├── compose.go         # Module composition
├── lm.go              # LanguageModel interface
├── lm_claude.go       # Claude SDK adapter
├── cache.go           # Response caching
├── trace.go           # Trace types
├── trace_store.go     # Trace persistence
├── evaluate.go        # Evaluation metrics
├── optimize.go        # Optimizer coordinator
├── bootstrap.go       # BootstrapFewShot
├── mipro.go           # MIPRO optimizer
├── knn.go             # KNN-based selection
├── messages.go        # Actor message types
├── service.go         # DSPyService behavior
├── actor.go           # Actor instantiation
└── *_test.go          # Tests for each file

internal/db/queries/
└── dspy.sql           # sqlc queries for DSPy tables

internal/mcp/
└── tools_dspy.go      # MCP tool handlers

cmd/substrate/
└── dspy.go            # CLI commands
```

---

## Next Steps

1. Review and approve this design document
2. Create tracking issues for Phase 1 tasks
3. Begin implementation with signature types
4. Set up CI for DSPy package tests
