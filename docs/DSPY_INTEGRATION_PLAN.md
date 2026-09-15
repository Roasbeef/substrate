# DSPy Integration Plan for Substrate

> **Status**: Design Phase
> **Author**: Claude
> **Date**: 2026-02-01

## Executive Summary

This document outlines a plan to integrate **actual DSPy** (the Python library) into Substrate via a Go wrapper/bridge architecture. Rather than recreating DSPy in Go, we leverage the mature Python DSPy library for all prompt engineering and optimization, while using Go for the actor system integration and the Claude Agent SDK as the inference backend.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Bridge Design Options](#bridge-design-options)
3. [Custom LM Backend](#custom-lm-backend)
4. [Go Wrapper Types](#go-wrapper-types)
5. [Python DSPy Server](#python-dspy-server)
6. [Implementation Phases](#implementation-phases)
7. [Example Usage](#example-usage)

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Go (Substrate)                                  │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                         Applications                                 │    │
│  │   (Agentic Review, Mail Summarization, Code Analysis, etc.)         │    │
│  └────────────────────────────────┬────────────────────────────────────┘    │
│                                   │                                          │
│  ┌────────────────────────────────▼────────────────────────────────────┐    │
│  │                      DSPy Go Client                                  │    │
│  │   - Type-safe wrapper structs (Signature, Module, Example)          │    │
│  │   - Actor message types for async execution                         │    │
│  │   - Marshals Go types ↔ JSON for Python bridge                      │    │
│  └────────────────────────────────┬────────────────────────────────────┘    │
│                                   │                                          │
│  ┌────────────────────────────────▼────────────────────────────────────┐    │
│  │                      Bridge Layer (gRPC/HTTP)                        │    │
│  │   - Connects to Python DSPy server                                  │    │
│  │   - Request/response serialization                                  │    │
│  │   - Connection pooling & health checks                              │    │
│  └────────────────────────────────┬────────────────────────────────────┘    │
│                                   │                                          │
│  ┌────────────────────────────────▼────────────────────────────────────┐    │
│  │                     LM Callback Server                               │    │
│  │   - HTTP server for DSPy LM callbacks                               │    │
│  │   - Routes inference requests to Claude Agent SDK                   │    │
│  │   - Spawner integration for actual Claude calls                     │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└──────────────────────────────────────┬───────────────────────────────────────┘
                                       │ gRPC / HTTP
                                       │
┌──────────────────────────────────────▼───────────────────────────────────────┐
│                              Python (DSPy Server)                             │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                        DSPy gRPC/HTTP Server                         │    │
│  │   - Exposes DSPy operations: predict, optimize, compile             │    │
│  │   - Manages DSPy modules and programs                               │    │
│  │   - Handles signature definitions                                   │    │
│  └────────────────────────────────┬────────────────────────────────────┘    │
│                                   │                                          │
│  ┌────────────────────────────────▼────────────────────────────────────┐    │
│  │                         Actual DSPy Library                          │    │
│  │   - dspy.Predict, dspy.ChainOfThought, dspy.ReAct                   │    │
│  │   - dspy.teleprompt.BootstrapFewShot, MIPRO, etc.                   │    │
│  │   - Full optimization and compilation                               │    │
│  └────────────────────────────────┬────────────────────────────────────┘    │
│                                   │                                          │
│  ┌────────────────────────────────▼────────────────────────────────────┐    │
│  │                      Custom LM (ClaudeAgentLM)                       │    │
│  │   - Subclass of dspy.LM                                             │    │
│  │   - Makes HTTP callbacks to Go LM server                            │    │
│  │   - Go server uses Claude Agent SDK for inference                   │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Key Design Principles

1. **Use Real DSPy** - Don't reinvent the wheel; DSPy's optimizers (MIPRO, BootstrapFewShot) are sophisticated
2. **Go for Orchestration** - Actor system, persistence, API exposure remain in Go
3. **Claude SDK as Backend** - Custom DSPy LM class calls back to Go for actual inference
4. **Type-Safe Go Wrapper** - Thin Go types that marshal to/from DSPy's Python types

---

## Bridge Design Options

### Option A: gRPC Bridge (Recommended)

```
Go Client  ←──gRPC──→  Python Server
```

**Pros:**
- Strong typing via protobuf
- Streaming support for long-running optimization
- Efficient binary serialization
- Built-in health checks and load balancing

**Cons:**
- Requires protobuf definitions
- More complex setup

### Option B: HTTP/JSON Bridge

```
Go Client  ←──HTTP/JSON──→  Python FastAPI Server
```

**Pros:**
- Simpler to implement and debug
- Easy to test with curl
- No protobuf tooling needed

**Cons:**
- Less efficient than gRPC
- No streaming (would need SSE or WebSocket)

### Option C: Subprocess with stdio JSON-RPC

```
Go  ──spawns──→  Python subprocess  ←──stdio JSON──→
```

**Pros:**
- No network configuration
- Process lifecycle managed by Go
- Simple for single-instance deployments

**Cons:**
- One process per Go instance
- Harder to scale horizontally

### Recommendation

**Start with HTTP/JSON** for simplicity during development, then migrate to **gRPC** for production. The Go wrapper types should be protocol-agnostic.

---

## Custom LM Backend

The key to using Claude Agent SDK with DSPy is a custom LM class that calls back to Go.

### Python: ClaudeAgentLM

```python
# dspy_server/lm.py

import dspy
import httpx
from typing import List, Optional

class ClaudeAgentLM(dspy.LM):
    """Custom DSPy LM that delegates inference to Go Claude Agent SDK."""

    def __init__(
        self,
        callback_url: str = "http://localhost:9090/v1/inference",
        model: str = "claude-opus-4-5-20251101",
        **kwargs
    ):
        super().__init__(model=model, **kwargs)
        self.callback_url = callback_url
        self.client = httpx.Client(timeout=300.0)  # 5 min for long generations

    def __call__(
        self,
        prompt: str,
        **kwargs
    ) -> List[str]:
        """Execute inference via Go callback server."""

        response = self.client.post(
            self.callback_url,
            json={
                "prompt": prompt,
                "model": self.model,
                "temperature": kwargs.get("temperature", 0.7),
                "max_tokens": kwargs.get("max_tokens", 4096),
                "n": kwargs.get("n", 1),  # Number of completions
            }
        )
        response.raise_for_status()

        result = response.json()

        # Track usage for cost monitoring
        self._update_usage(result.get("usage", {}))

        # Return list of completions
        return result["completions"]

    def _update_usage(self, usage: dict):
        """Track token usage and costs."""
        # DSPy tracks this internally
        pass


# Configure DSPy to use our custom LM
def configure_dspy(callback_url: str, model: str = "claude-opus-4-5-20251101"):
    """Initialize DSPy with Claude Agent backend."""
    lm = ClaudeAgentLM(callback_url=callback_url, model=model)
    dspy.configure(lm=lm)
    return lm
```

### Go: LM Callback Server

```go
// internal/dspy/lm_server.go

package dspy

import (
    "context"
    "encoding/json"
    "net/http"

    "github.com/Roasbeef/substrate/internal/agent"
)

// LMCallbackServer handles inference requests from Python DSPy.
type LMCallbackServer struct {
    spawner *agent.Spawner
    addr    string
}

// InferenceRequest from Python DSPy LM.
type InferenceRequest struct {
    Prompt      string  `json:"prompt"`
    Model       string  `json:"model"`
    Temperature float64 `json:"temperature"`
    MaxTokens   int     `json:"max_tokens"`
    N           int     `json:"n"`  // Number of completions
}

// InferenceResponse to Python DSPy LM.
type InferenceResponse struct {
    Completions []string       `json:"completions"`
    Usage       *UsageInfo     `json:"usage,omitempty"`
    Error       string         `json:"error,omitempty"`
}

// UsageInfo tracks token usage for cost monitoring.
type UsageInfo struct {
    PromptTokens     int     `json:"prompt_tokens"`
    CompletionTokens int     `json:"completion_tokens"`
    TotalTokens      int     `json:"total_tokens"`
    CostUSD          float64 `json:"cost_usd"`
}

// NewLMCallbackServer creates a new callback server.
func NewLMCallbackServer(spawner *agent.Spawner, addr string) *LMCallbackServer {
    return &LMCallbackServer{
        spawner: spawner,
        addr:    addr,
    }
}

// Start begins serving inference requests.
func (s *LMCallbackServer) Start(ctx context.Context) error {
    mux := http.NewServeMux()
    mux.HandleFunc("/v1/inference", s.handleInference)
    mux.HandleFunc("/health", s.handleHealth)

    server := &http.Server{
        Addr:    s.addr,
        Handler: mux,
    }

    go func() {
        <-ctx.Done()
        server.Shutdown(context.Background())
    }()

    return server.ListenAndServe()
}

// handleInference processes a single inference request.
func (s *LMCallbackServer) handleInference(w http.ResponseWriter, r *http.Request) {
    var req InferenceRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Configure spawner for this request.
    cfg := agent.DefaultSpawnConfig()
    cfg.Model = req.Model

    spawner := agent.NewSpawner(cfg)

    // Generate N completions (for optimization scenarios).
    completions := make([]string, 0, req.N)
    var totalUsage UsageInfo

    for i := 0; i < req.N; i++ {
        resp, err := spawner.Spawn(r.Context(), req.Prompt)
        if err != nil {
            json.NewEncoder(w).Encode(InferenceResponse{
                Error: err.Error(),
            })
            return
        }

        completions = append(completions, resp.Result)

        if resp.Usage != nil {
            totalUsage.PromptTokens += int(resp.Usage.InputTokens)
            totalUsage.CompletionTokens += int(resp.Usage.OutputTokens)
            totalUsage.CostUSD += resp.CostUSD
        }
    }

    totalUsage.TotalTokens = totalUsage.PromptTokens + totalUsage.CompletionTokens

    json.NewEncoder(w).Encode(InferenceResponse{
        Completions: completions,
        Usage:       &totalUsage,
    })
}

func (s *LMCallbackServer) handleHealth(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("ok"))
}
```

---

## Go Wrapper Types

Type-safe Go structs that map to DSPy concepts.

```go
// internal/dspy/types.go

package dspy

import "time"

// Signature defines input/output fields for a DSPy module.
// Maps to: dspy.Signature
type Signature struct {
    Name         string  `json:"name"`
    Description  string  `json:"description"`
    Instructions string  `json:"instructions,omitempty"`
    InputFields  []Field `json:"input_fields"`
    OutputFields []Field `json:"output_fields"`
}

// Field represents a single input or output field.
// Maps to: dspy.InputField / dspy.OutputField
type Field struct {
    Name        string `json:"name"`
    Description string `json:"desc,omitempty"`
    Prefix      string `json:"prefix,omitempty"`  // For output parsing
    Type        string `json:"type,omitempty"`    // "str", "int", "list", etc.
}

// Example is a single input/output demonstration.
// Maps to: dspy.Example
type Example struct {
    Inputs  map[string]any `json:"inputs"`
    Outputs map[string]any `json:"outputs,omitempty"`
}

// ModuleType identifies the DSPy module to use.
type ModuleType string

const (
    ModulePredict        ModuleType = "Predict"
    ModuleChainOfThought ModuleType = "ChainOfThought"
    ModuleReAct          ModuleType = "ReAct"
    ModuleProgramOfThought ModuleType = "ProgramOfThought"
)

// PredictRequest asks DSPy to run a prediction.
type PredictRequest struct {
    Signature  Signature      `json:"signature"`
    ModuleType ModuleType     `json:"module_type"`
    Inputs     map[string]any `json:"inputs"`
    Config     *PredictConfig `json:"config,omitempty"`
}

// PredictConfig contains optional prediction parameters.
type PredictConfig struct {
    Temperature float64 `json:"temperature,omitempty"`
    MaxTokens   int     `json:"max_tokens,omitempty"`
}

// PredictResponse contains the prediction result.
type PredictResponse struct {
    Outputs   map[string]any `json:"outputs"`
    Rationale string         `json:"rationale,omitempty"`  // If CoT
    TraceID   string         `json:"trace_id,omitempty"`
    Usage     *UsageInfo     `json:"usage,omitempty"`
}

// OptimizeStrategy identifies the teleprompter to use.
type OptimizeStrategy string

const (
    StrategyBootstrapFewShot     OptimizeStrategy = "BootstrapFewShot"
    StrategyBootstrapFewShotRS   OptimizeStrategy = "BootstrapFewShotWithRandomSearch"
    StrategyMIPRO                OptimizeStrategy = "MIPRO"
    StrategyMIPROv2              OptimizeStrategy = "MIPROv2"
    StrategyKNNFewShot           OptimizeStrategy = "KNNFewShot"
    StrategyCOPRO                OptimizeStrategy = "COPRO"
)

// OptimizeRequest asks DSPy to optimize a module.
type OptimizeRequest struct {
    Signature    Signature        `json:"signature"`
    ModuleType   ModuleType       `json:"module_type"`
    Strategy     OptimizeStrategy `json:"strategy"`
    TrainSet     []Example        `json:"train_set"`
    ValSet       []Example        `json:"val_set,omitempty"`
    Metric       string           `json:"metric"`  // Python metric function name
    Config       *OptimizeConfig  `json:"config,omitempty"`
}

// OptimizeConfig contains optimization parameters.
type OptimizeConfig struct {
    MaxBootstrappedDemos int     `json:"max_bootstrapped_demos,omitempty"`
    MaxLabeledDemos      int     `json:"max_labeled_demos,omitempty"`
    NumCandidates        int     `json:"num_candidates,omitempty"`
    MaxErrors            int     `json:"max_errors,omitempty"`
    Temperature          float64 `json:"temperature,omitempty"`
}

// OptimizeResponse contains the optimized program.
type OptimizeResponse struct {
    // The optimized program can be serialized and reloaded.
    ProgramJSON string  `json:"program_json"`

    // Metrics from optimization.
    TrainScore  float64 `json:"train_score"`
    ValScore    float64 `json:"val_score,omitempty"`

    // Selected examples after optimization.
    SelectedDemos []Example `json:"selected_demos"`

    // Cost tracking.
    TotalCost   float64       `json:"total_cost"`
    TotalTokens int           `json:"total_tokens"`
    Duration    time.Duration `json:"duration"`
}

// CompileRequest asks DSPy to compile a program for inference.
type CompileRequest struct {
    ProgramJSON string `json:"program_json"`
}

// CompileResponse returns the compiled program ID.
type CompileResponse struct {
    ProgramID string `json:"program_id"`
}
```

---

## Python DSPy Server

The Python server that wraps actual DSPy.

### Project Structure

```
dspy_server/
├── pyproject.toml
├── dspy_server/
│   ├── __init__.py
│   ├── server.py          # FastAPI/gRPC server
│   ├── lm.py              # ClaudeAgentLM custom backend
│   ├── modules.py         # Module factory and registry
│   ├── metrics.py         # Common metric functions
│   └── programs.py        # Program compilation and storage
└── tests/
    └── test_server.py
```

### FastAPI Server Implementation

```python
# dspy_server/server.py

import dspy
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import Dict, Any, List, Optional
import json
import uuid

from .lm import ClaudeAgentLM, configure_dspy
from .modules import create_module, ModuleType
from .metrics import get_metric

app = FastAPI(title="DSPy Server", version="1.0.0")

# In-memory program storage (use Redis/DB in production)
_programs: Dict[str, dspy.Module] = {}


class Field(BaseModel):
    name: str
    desc: Optional[str] = None
    prefix: Optional[str] = None
    type: Optional[str] = "str"


class Signature(BaseModel):
    name: str
    description: str
    instructions: Optional[str] = None
    input_fields: List[Field]
    output_fields: List[Field]


class Example(BaseModel):
    inputs: Dict[str, Any]
    outputs: Optional[Dict[str, Any]] = None


class PredictConfig(BaseModel):
    temperature: Optional[float] = 0.7
    max_tokens: Optional[int] = 4096


class PredictRequest(BaseModel):
    signature: Signature
    module_type: str = "Predict"
    inputs: Dict[str, Any]
    config: Optional[PredictConfig] = None


class PredictResponse(BaseModel):
    outputs: Dict[str, Any]
    rationale: Optional[str] = None
    trace_id: Optional[str] = None


class OptimizeConfig(BaseModel):
    max_bootstrapped_demos: Optional[int] = 4
    max_labeled_demos: Optional[int] = 4
    num_candidates: Optional[int] = 10
    max_errors: Optional[int] = 5
    temperature: Optional[float] = 0.7


class OptimizeRequest(BaseModel):
    signature: Signature
    module_type: str = "Predict"
    strategy: str = "BootstrapFewShot"
    train_set: List[Example]
    val_set: Optional[List[Example]] = None
    metric: str = "exact_match"
    config: Optional[OptimizeConfig] = None


class OptimizeResponse(BaseModel):
    program_json: str
    train_score: float
    val_score: Optional[float] = None
    selected_demos: List[Example]
    total_cost: float
    total_tokens: int
    duration: float


@app.on_event("startup")
async def startup():
    """Configure DSPy on server start."""
    # The callback URL is where Go's LM server listens
    configure_dspy(
        callback_url="http://localhost:9090/v1/inference",
        model="claude-opus-4-5-20251101"
    )


def signature_to_dspy(sig: Signature) -> dspy.Signature:
    """Convert API signature to DSPy signature."""
    # Build signature string: "input1, input2 -> output1, output2"
    inputs = ", ".join(f.name for f in sig.input_fields)
    outputs = ", ".join(f.name for f in sig.output_fields)
    sig_str = f"{inputs} -> {outputs}"

    # Create signature class dynamically
    class DynamicSignature(dspy.Signature):
        pass

    DynamicSignature.__doc__ = sig.description
    if sig.instructions:
        DynamicSignature.__doc__ += f"\n\n{sig.instructions}"

    # Add input fields
    for field in sig.input_fields:
        setattr(DynamicSignature, field.name, dspy.InputField(
            desc=field.desc or field.name
        ))

    # Add output fields
    for field in sig.output_fields:
        setattr(DynamicSignature, field.name, dspy.OutputField(
            desc=field.desc or field.name,
            prefix=field.prefix or f"{field.name}:"
        ))

    return DynamicSignature


@app.post("/predict", response_model=PredictResponse)
async def predict(request: PredictRequest):
    """Execute a DSPy prediction."""
    try:
        # Convert signature
        sig_class = signature_to_dspy(request.signature)

        # Create module
        module = create_module(request.module_type, sig_class)

        # Create example from inputs
        example = dspy.Example(**request.inputs).with_inputs(*request.inputs.keys())

        # Run prediction
        with dspy.context(temperature=request.config.temperature if request.config else 0.7):
            result = module(example)

        # Extract outputs
        outputs = {}
        for field in request.signature.output_fields:
            if hasattr(result, field.name):
                outputs[field.name] = getattr(result, field.name)

        # Extract rationale for CoT
        rationale = None
        if hasattr(result, "rationale"):
            rationale = result.rationale

        return PredictResponse(
            outputs=outputs,
            rationale=rationale,
            trace_id=str(uuid.uuid4())
        )

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/optimize", response_model=OptimizeResponse)
async def optimize(request: OptimizeRequest):
    """Optimize a DSPy module using a teleprompter."""
    import time
    start_time = time.time()

    try:
        # Convert signature
        sig_class = signature_to_dspy(request.signature)

        # Create module
        module = create_module(request.module_type, sig_class)

        # Convert examples to DSPy format
        trainset = [
            dspy.Example(**ex.inputs, **ex.outputs).with_inputs(*ex.inputs.keys())
            for ex in request.train_set
        ]

        valset = None
        if request.val_set:
            valset = [
                dspy.Example(**ex.inputs, **ex.outputs).with_inputs(*ex.inputs.keys())
                for ex in request.val_set
            ]

        # Get metric function
        metric = get_metric(request.metric)

        # Get teleprompter
        config = request.config or OptimizeConfig()
        teleprompter = get_teleprompter(
            request.strategy,
            metric=metric,
            max_bootstrapped_demos=config.max_bootstrapped_demos,
            max_labeled_demos=config.max_labeled_demos,
            num_candidates=config.num_candidates,
        )

        # Run optimization
        optimized = teleprompter.compile(
            module,
            trainset=trainset,
            valset=valset,
        )

        # Evaluate
        from dspy.evaluate import Evaluate
        evaluator = Evaluate(devset=trainset, metric=metric)
        train_score = evaluator(optimized)

        val_score = None
        if valset:
            val_evaluator = Evaluate(devset=valset, metric=metric)
            val_score = val_evaluator(optimized)

        # Serialize program
        program_json = optimized.save(path=None)  # Returns JSON string

        # Extract selected demos
        selected_demos = []
        if hasattr(optimized, "demos"):
            for demo in optimized.demos:
                selected_demos.append(Example(
                    inputs={k: getattr(demo, k) for k in demo._input_keys},
                    outputs={k: getattr(demo, k) for k in demo._output_keys if hasattr(demo, k)}
                ))

        duration = time.time() - start_time

        return OptimizeResponse(
            program_json=program_json,
            train_score=train_score,
            val_score=val_score,
            selected_demos=selected_demos,
            total_cost=0.0,  # TODO: Track from LM
            total_tokens=0,  # TODO: Track from LM
            duration=duration
        )

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


def get_teleprompter(strategy: str, **kwargs):
    """Get the appropriate teleprompter by name."""
    from dspy.teleprompt import (
        BootstrapFewShot,
        BootstrapFewShotWithRandomSearch,
        MIPRO,
        KNNFewShot,
        COPRO,
    )

    teleprompters = {
        "BootstrapFewShot": BootstrapFewShot,
        "BootstrapFewShotWithRandomSearch": BootstrapFewShotWithRandomSearch,
        "MIPRO": MIPRO,
        "KNNFewShot": KNNFewShot,
        "COPRO": COPRO,
    }

    tp_class = teleprompters.get(strategy)
    if not tp_class:
        raise ValueError(f"Unknown strategy: {strategy}")

    return tp_class(**kwargs)


@app.post("/compile")
async def compile_program(program_json: str):
    """Load and compile a saved program for fast inference."""
    program_id = str(uuid.uuid4())

    # Load the program
    program = dspy.Module()
    program.load(path=None, data=program_json)

    _programs[program_id] = program

    return {"program_id": program_id}


@app.post("/infer/{program_id}")
async def infer(program_id: str, inputs: Dict[str, Any]):
    """Run inference on a compiled program."""
    if program_id not in _programs:
        raise HTTPException(status_code=404, detail="Program not found")

    program = _programs[program_id]
    example = dspy.Example(**inputs).with_inputs(*inputs.keys())

    result = program(example)

    return {"outputs": dict(result)}


@app.get("/health")
async def health():
    return {"status": "ok"}
```

### Module Factory

```python
# dspy_server/modules.py

import dspy


def create_module(module_type: str, signature) -> dspy.Module:
    """Create a DSPy module by type name."""

    if module_type == "Predict":
        return dspy.Predict(signature)

    elif module_type == "ChainOfThought":
        return dspy.ChainOfThought(signature)

    elif module_type == "ReAct":
        # ReAct requires tools - would need to extend API
        return dspy.ReAct(signature, tools=[])

    elif module_type == "ProgramOfThought":
        return dspy.ProgramOfThought(signature)

    else:
        raise ValueError(f"Unknown module type: {module_type}")
```

### Common Metrics

```python
# dspy_server/metrics.py

import dspy


def exact_match(example, prediction, trace=None):
    """Check if prediction exactly matches expected output."""
    for key in example._output_keys:
        expected = getattr(example, key, None)
        predicted = getattr(prediction, key, None)
        if expected != predicted:
            return False
    return True


def contains_match(example, prediction, trace=None):
    """Check if expected output is contained in prediction."""
    for key in example._output_keys:
        expected = str(getattr(example, key, ""))
        predicted = str(getattr(prediction, key, ""))
        if expected.lower() not in predicted.lower():
            return False
    return True


def llm_as_judge(example, prediction, trace=None):
    """Use LLM to judge quality (expensive but flexible)."""
    judge = dspy.ChainOfThought("context, expected, predicted -> score: float")
    result = judge(
        context=str(example),
        expected=str({k: getattr(example, k) for k in example._output_keys}),
        predicted=str({k: getattr(prediction, k) for k in example._output_keys if hasattr(prediction, k)})
    )
    return float(result.score) >= 0.5


_METRICS = {
    "exact_match": exact_match,
    "contains_match": contains_match,
    "llm_as_judge": llm_as_judge,
}


def get_metric(name: str):
    """Get metric function by name."""
    if name not in _METRICS:
        raise ValueError(f"Unknown metric: {name}. Available: {list(_METRICS.keys())}")
    return _METRICS[name]
```

---

## Go DSPy Client

The Go client that calls the Python server.

```go
// internal/dspy/client.go

package dspy

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// Client communicates with the Python DSPy server.
type Client struct {
    baseURL    string
    httpClient *http.Client
}

// NewClient creates a new DSPy client.
func NewClient(baseURL string) *Client {
    return &Client{
        baseURL: baseURL,
        httpClient: &http.Client{
            Timeout: 10 * time.Minute, // Long timeout for optimization
        },
    }
}

// Predict executes a prediction using DSPy.
func (c *Client) Predict(ctx context.Context, req *PredictRequest) (*PredictResponse, error) {
    return doRequest[PredictResponse](ctx, c, "POST", "/predict", req)
}

// Optimize runs prompt optimization using DSPy teleprompters.
func (c *Client) Optimize(ctx context.Context, req *OptimizeRequest) (*OptimizeResponse, error) {
    return doRequest[OptimizeResponse](ctx, c, "POST", "/optimize", req)
}

// Compile loads a saved program for fast inference.
func (c *Client) Compile(ctx context.Context, req *CompileRequest) (*CompileResponse, error) {
    return doRequest[CompileResponse](ctx, c, "POST", "/compile", req)
}

// Infer runs inference on a compiled program.
func (c *Client) Infer(ctx context.Context, programID string, inputs map[string]any) (map[string]any, error) {
    resp, err := doRequest[map[string]any](ctx, c, "POST", fmt.Sprintf("/infer/%s", programID), inputs)
    if err != nil {
        return nil, err
    }
    return *resp, nil
}

// Health checks if the DSPy server is running.
func (c *Client) Health(ctx context.Context) error {
    _, err := doRequest[map[string]any](ctx, c, "GET", "/health", nil)
    return err
}

// doRequest is a generic HTTP request helper.
func doRequest[T any](ctx context.Context, c *Client, method, path string, body any) (*T, error) {
    var reqBody []byte
    var err error

    if body != nil {
        reqBody, err = json.Marshal(body)
        if err != nil {
            return nil, fmt.Errorf("marshal request: %w", err)
        }
    }

    req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(reqBody))
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("do request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        var errResp struct {
            Detail string `json:"detail"`
        }
        json.NewDecoder(resp.Body).Decode(&errResp)
        return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, errResp.Detail)
    }

    var result T
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, fmt.Errorf("decode response: %w", err)
    }

    return &result, nil
}
```

---

## Actor Integration

Wrap the DSPy client in an actor for async execution.

```go
// internal/dspy/actor.go

package dspy

import (
    "context"
    "fmt"

    "github.com/Roasbeef/substrate/internal/baselib/actor"
    "github.com/lightningnetwork/lnd/fn/v2"
)

// DSPyRequest is the sealed union of all DSPy actor requests.
type DSPyRequest interface {
    actor.Message
    isDSPyRequest()
}

// DSPyResponse is the sealed union of all DSPy actor responses.
type DSPyResponse interface {
    isDSPyResponse()
}

// Ensure request types implement the interface.
func (PredictRequest) isDSPyRequest()  {}
func (OptimizeRequest) isDSPyRequest() {}

// Ensure response types implement the interface.
func (PredictResponse) isDSPyResponse()  {}
func (OptimizeResponse) isDSPyResponse() {}

// Service implements actor behavior for DSPy operations.
type Service struct {
    client *Client
}

// NewService creates a new DSPy service.
func NewService(client *Client) *Service {
    return &Service{client: client}
}

// Receive handles incoming DSPy requests.
func (s *Service) Receive(ctx context.Context, msg DSPyRequest) fn.Result[DSPyResponse] {
    switch m := msg.(type) {
    case PredictRequest:
        resp, err := s.client.Predict(ctx, &m)
        if err != nil {
            return fn.Err[DSPyResponse](err)
        }
        return fn.Ok[DSPyResponse](*resp)

    case OptimizeRequest:
        resp, err := s.client.Optimize(ctx, &m)
        if err != nil {
            return fn.Err[DSPyResponse](err)
        }
        return fn.Ok[DSPyResponse](*resp)

    default:
        return fn.Err[DSPyResponse](fmt.Errorf("unknown request type: %T", msg))
    }
}

// DSPyActorConfig configures the DSPy actor.
type DSPyActorConfig struct {
    ID          string
    Client      *Client
    MailboxSize int
}

// NewDSPyActor creates a new DSPy actor.
func NewDSPyActor(cfg DSPyActorConfig) *actor.Actor[DSPyRequest, DSPyResponse] {
    svc := NewService(cfg.Client)
    return actor.NewActor(actor.ActorConfig[DSPyRequest, DSPyResponse]{
        ID:          cfg.ID,
        Behavior:    svc,
        MailboxSize: cfg.MailboxSize,
    })
}

// StartDSPyActor creates and starts a DSPy actor.
func StartDSPyActor(cfg DSPyActorConfig) *actor.ActorRef[DSPyRequest, DSPyResponse] {
    a := NewDSPyActor(cfg)
    a.Start()
    return a.Ref()
}
```

---

## Implementation Phases

### Phase 1: Bridge Foundation (Week 1)

| Task | Description | Deliverable |
|------|-------------|-------------|
| 1.1 | Set up Python DSPy server project | `dspy_server/` |
| 1.2 | Implement ClaudeAgentLM custom backend | `dspy_server/lm.py` |
| 1.3 | Implement Go LM callback server | `internal/dspy/lm_server.go` |
| 1.4 | Basic predict endpoint (Python) | `dspy_server/server.py` |
| 1.5 | Go client types and HTTP client | `internal/dspy/client.go` |
| 1.6 | Integration test: Go → Python → Go → Claude | `tests/integration/dspy/` |

### Phase 2: Full DSPy Features (Week 2)

| Task | Description | Deliverable |
|------|-------------|-------------|
| 2.1 | ChainOfThought, ReAct modules | `dspy_server/modules.py` |
| 2.2 | Optimization endpoints (BootstrapFewShot) | `dspy_server/server.py` |
| 2.3 | Program save/load/compile | `dspy_server/programs.py` |
| 2.4 | Common metrics | `dspy_server/metrics.py` |
| 2.5 | Go client optimization methods | `internal/dspy/client.go` |

### Phase 3: Actor Integration (Week 3)

| Task | Description | Deliverable |
|------|-------------|-------------|
| 3.1 | DSPy actor message types | `internal/dspy/messages.go` |
| 3.2 | DSPy actor service | `internal/dspy/actor.go` |
| 3.3 | MCP tools for DSPy | `internal/mcp/tools_dspy.go` |
| 3.4 | CLI commands | `cmd/substrate/dspy.go` |
| 3.5 | Process management (start/stop Python server) | `internal/dspy/process.go` |

### Phase 4: Applications (Week 4)

| Task | Description | Deliverable |
|------|-------------|-------------|
| 4.1 | Agentic Review using DSPy | `internal/apps/review/` |
| 4.2 | Mail summarization | `internal/apps/summarize/` |
| 4.3 | Persistence (save optimized programs to DB) | `internal/store/dspy.go` |
| 4.4 | Web UI for managing signatures | `internal/web/templates/dspy/` |

---

## Example Usage

### Go Application Code

```go
// Example: Using DSPy for code review

func (r *ReviewService) Review(ctx context.Context, code string) (*ReviewResult, error) {
    // Define the signature
    sig := dspy.Signature{
        Name:        "code_review",
        Description: "Review code and provide actionable feedback.",
        InputFields: []dspy.Field{
            {Name: "code", Description: "The code to review"},
            {Name: "language", Description: "Programming language"},
        },
        OutputFields: []dspy.Field{
            {Name: "issues", Description: "List of issues found", Prefix: "Issues:"},
            {Name: "suggestions", Description: "Improvement suggestions", Prefix: "Suggestions:"},
            {Name: "score", Description: "Quality score 1-10", Prefix: "Score:"},
        },
    }

    // Run prediction through DSPy actor
    req := dspy.PredictRequest{
        Signature:  sig,
        ModuleType: dspy.ModuleChainOfThought,  // Use CoT for reasoning
        Inputs: map[string]any{
            "code":     code,
            "language": detectLanguage(code),
        },
    }

    // Send to actor and await response
    future := r.dspyActor.Ask(ctx, req)
    resp, err := future.Await(ctx)
    if err != nil {
        return nil, err
    }

    predictResp := resp.(dspy.PredictResponse)

    return &ReviewResult{
        Issues:      predictResp.Outputs["issues"].(string),
        Suggestions: predictResp.Outputs["suggestions"].(string),
        Score:       predictResp.Outputs["score"].(string),
        Rationale:   predictResp.Rationale,
    }, nil
}
```

### Running Optimization

```go
// Optimize the code review module with examples

func (r *ReviewService) OptimizeReviewer(ctx context.Context) error {
    // Load training examples from database
    examples, err := r.store.GetReviewExamples(ctx, 100)
    if err != nil {
        return err
    }

    // Convert to DSPy examples
    trainSet := make([]dspy.Example, len(examples))
    for i, ex := range examples {
        trainSet[i] = dspy.Example{
            Inputs: map[string]any{
                "code":     ex.Code,
                "language": ex.Language,
            },
            Outputs: map[string]any{
                "issues":      ex.Issues,
                "suggestions": ex.Suggestions,
                "score":       ex.Score,
            },
        }
    }

    // Split into train/val
    trainSet, valSet := splitExamples(trainSet, 0.8)

    // Run optimization
    req := dspy.OptimizeRequest{
        Signature:  codeReviewSignature,
        ModuleType: dspy.ModuleChainOfThought,
        Strategy:   dspy.StrategyMIPRO,
        TrainSet:   trainSet,
        ValSet:     valSet,
        Metric:     "llm_as_judge",
        Config: &dspy.OptimizeConfig{
            MaxBootstrappedDemos: 4,
            NumCandidates:        10,
        },
    }

    future := r.dspyActor.Ask(ctx, req)
    resp, err := future.Await(ctx)
    if err != nil {
        return err
    }

    optResp := resp.(dspy.OptimizeResponse)

    // Save optimized program
    return r.store.SaveOptimizedProgram(ctx, "code_review", optResp.ProgramJSON)
}
```

---

## Deployment

### Docker Compose

```yaml
version: "3.8"

services:
  substrate:
    build: .
    ports:
      - "8080:8080"   # Web UI
      - "9090:9090"   # LM callback server
    environment:
      - DSPY_SERVER_URL=http://dspy:8000
      - LM_CALLBACK_ADDR=:9090
    depends_on:
      - dspy

  dspy:
    build: ./dspy_server
    ports:
      - "8000:8000"
    environment:
      - LM_CALLBACK_URL=http://substrate:9090/v1/inference
```

### Process Management (Single Binary)

For simpler deployments, embed Python server management in Go:

```go
// internal/dspy/process.go

type DSPyProcess struct {
    cmd     *exec.Cmd
    baseURL string
}

func StartDSPyServer(ctx context.Context, port int) (*DSPyProcess, error) {
    cmd := exec.CommandContext(ctx, "python", "-m", "dspy_server", "--port", fmt.Sprint(port))
    cmd.Env = append(os.Environ(),
        fmt.Sprintf("LM_CALLBACK_URL=http://localhost:%d/v1/inference", lmPort),
    )

    if err := cmd.Start(); err != nil {
        return nil, err
    }

    // Wait for health check
    baseURL := fmt.Sprintf("http://localhost:%d", port)
    if err := waitForHealth(ctx, baseURL); err != nil {
        cmd.Process.Kill()
        return nil, err
    }

    return &DSPyProcess{cmd: cmd, baseURL: baseURL}, nil
}
```

---

## Summary

This revised plan uses **actual DSPy** via a Python server with:

1. **Custom LM Backend** (`ClaudeAgentLM`) that calls back to Go
2. **Go Wrapper Types** for type safety without reimplementing DSPy
3. **HTTP Bridge** (upgradable to gRPC) between Go and Python
4. **Actor Integration** for async execution in Substrate
5. **Full DSPy Features** - optimization, compilation, all module types

This approach gives us DSPy's sophisticated prompt optimization (MIPRO, BootstrapFewShot, etc.) while keeping orchestration in Go and using the Claude Agent SDK for inference.
