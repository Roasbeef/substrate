# Roadmap Tasks Generator

Generate Tasks from the project roadmap for the next implementation phases.

## Usage

```
/roadmap-tasks [--phase <A|B|C|all>] [--items <item-ids>]
```

**Options:**
- `--phase A` - Create tasks for Phase A (Functional Core / Imperative Shell)
- `--phase B` - Create tasks for Phase B (Database Actor Layer)
- `--phase C` - Create tasks for Phase C (Test Coverage Improvements)
- `--phase all` - Show all roadmap items and let user select
- `--items A1,A2,B1` - Create tasks for specific items by ID

## Steps

1. **Read the Roadmap:**
   ```bash
   cat docs/ROADMAP.md
   ```

2. **Parse Roadmap Items:**
   Extract unchecked items (`- [ ]`) from the Architecture Improvements section:
   - Phase A: A1-A5 (Functional Core / Imperative Shell)
   - Phase B: B1-B6 (Database Actor Layer)
   - Phase C: C1-C6 (Test Coverage Improvements)

3. **Present Options to User:**
   If no specific phase/items requested, show available items and ask which to create:
   ```
   ## Available Roadmap Items

   ### Phase A: Functional Core / Imperative Shell
   - [ ] A1: Extract pure functions from handlers into `internal/web/logic/` package
   - [ ] A2: Define interfaces for all external dependencies (db, mail service)
   ...

   ### Phase B: Database Actor Layer
   - [ ] B1: Design `DBWorker` actor with request/response message types
   ...

   Which items would you like to create Tasks for?
   ```

4. **Create Tasks:**
   For each selected item, use `TaskCreate` with:
   - **subject**: The item description (e.g., "Extract pure functions from handlers")
   - **description**: Expanded details including:
     - Context from the roadmap
     - Acceptance criteria
     - Related files/packages
   - **activeForm**: Present continuous (e.g., "Extracting pure functions from handlers")
   - **metadata**: Include roadmap_id (e.g., "A1"), phase, priority, size estimate

5. **Set Dependencies:**
   After creating tasks, use `TaskUpdate` to set logical dependencies:
   - A2 blocked by A1 (need pure functions before interfaces)
   - A3 blocked by A2 (need interfaces before mocks)
   - B3 blocked by B1 (need actor design before typed wrappers)
   - etc.

## Task Templates

### Phase A Tasks

**A1: Extract pure functions**
```
Subject: Extract pure functions from handlers into internal/web/logic/ package
Description: |
  Refactor web handlers to separate pure business logic from I/O operations.

  ## Goal
  Move all pure functions (no side effects, no I/O) from handlers.go into
  a new `internal/web/logic/` package.

  ## Acceptance Criteria
  - [ ] Create `internal/web/logic/` package
  - [ ] Identify pure functions in handlers.go (data transformation, validation, formatting)
  - [ ] Extract each pure function with comprehensive unit tests
  - [ ] Handlers only call pure functions + I/O operations
  - [ ] 90%+ test coverage for logic package

  ## Files
  - internal/web/handlers.go (source)
  - internal/web/logic/*.go (new)

Priority: P1
Size: M
```

**A2: Define dependency interfaces**
```
Subject: Define interfaces for all external dependencies
Description: |
  Create interfaces for database and mail service dependencies to enable testing.

  ## Acceptance Criteria
  - [ ] Define `Store` interface in internal/web/logic/ (or internal/web/deps/)
  - [ ] Define `MailService` interface
  - [ ] Update handlers to accept interfaces instead of concrete types
  - [ ] Document interface contracts

Priority: P1
Size: S
```

### Phase B Tasks

**B1: Design DBWorker actor**
```
Subject: Design DBWorker actor with request/response message types
Description: |
  Design the actor-based database access layer following darepo-client patterns.

  ## Goal
  Create a `DBWorker` actor that handles database queries via message passing,
  providing better concurrency control and potential connection pooling.

  ## Acceptance Criteria
  - [ ] Define message types for all query operations
  - [ ] Design actor interface following darepo-client patterns
  - [ ] Document the actor lifecycle and message flow
  - [ ] Create design doc in docs/DB_ACTOR.md

Priority: P2
Size: S
```

### Phase C Tasks

**C1: Database layer unit tests**
```
Subject: Add unit tests for database layer (target 85%+)
Description: |
  Increase test coverage for the database layer.

  ## Current Coverage
  - internal/db: 73.4%
  - Target: 85%+

  ## Acceptance Criteria
  - [ ] Test all CRUD operations
  - [ ] Test error paths (not found, constraint violations)
  - [ ] Test FTS5 search functionality
  - [ ] Test transaction handling
  - [ ] Coverage report shows 85%+

Priority: P1
Size: M
```

## Output Format

After creating tasks:
```
## Created Tasks from Roadmap

| Task ID | Roadmap ID | Subject | Priority | Size |
|---------|------------|---------|----------|------|
| #1 | A1 | Extract pure functions from handlers | P1 | M |
| #2 | A2 | Define interfaces for dependencies | P1 | S |
| #3 | A3 | Create mock implementations | P1 | S |

### Dependencies Set
- #2 blocked by #1
- #3 blocked by #2

Next: Run `/task-next` to start working on the first available task.
```
