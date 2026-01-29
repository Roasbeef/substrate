# Future Plans

## Vector Search with sqlite-vec

For future semantic/vector-based search capabilities, we can integrate
[sqlite-vec](https://github.com/asg017/sqlite-vec) - a SQLite extension for
vector search.

This would enable:
- Semantic search across messages (find similar content, not just keyword matches)
- Agent recommendation based on message embeddings
- Topic clustering and discovery
- Smart routing based on message similarity

### Integration Notes

sqlite-vec provides:
- Virtual tables for vector storage
- Efficient KNN (k-nearest neighbors) queries
- Support for various distance metrics (L2, cosine, etc.)
- Pure C implementation with no dependencies

### Example Usage

```sql
-- Create a vector table for message embeddings
CREATE VIRTUAL TABLE message_embeddings USING vec0(
    message_id INTEGER PRIMARY KEY,
    embedding FLOAT[384]  -- Dimension depends on embedding model
);

-- Insert embeddings (from Go code with an embedding model)
INSERT INTO message_embeddings (message_id, embedding)
VALUES (1, ?);

-- Find similar messages
SELECT m.*, distance
FROM message_embeddings me
JOIN messages m ON me.message_id = m.id
WHERE me.embedding MATCH ?
ORDER BY distance
LIMIT 10;
```

### Prerequisites

1. Install sqlite-vec extension
2. Integrate an embedding model (e.g., sentence-transformers)
3. Create embedding pipeline for new messages
4. Add vector search queries to the store
