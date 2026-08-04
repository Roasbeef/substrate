#!/usr/bin/env python3
"""A stand-in for the claude CLI, used to test the generator's wiring.

The generator's behaviour depends on flags that reach the real CLI's command
line, and on messages that come back over its stream-json protocol. Neither can
be observed from Go alone: the SDK builds the argv internally, and exercising
the real binary needs credentials and costs money on every run.

This stub closes both gaps. It records the argv it was invoked with so a test
can assert on the flags, answers the initialize handshake, and then replays a
scripted sequence of protocol messages so a test can drive the generator down a
chosen path -- a clean plan, an API retry storm, a hook that eats the turn --
deterministically and offline.
"""

import json
import os
import sys


def emit(obj):
    """Write one protocol message to stdout, flushing so Go sees it promptly."""
    sys.stdout.write(json.dumps(obj) + "\n")
    sys.stdout.flush()


def main():
    argv_out = os.environ.get("STUB_ARGV_OUT")
    if argv_out:
        with open(argv_out, "w") as handle:
            handle.write("\n".join(sys.argv[1:]))

    # The scenario names which scripted stream to replay once the SDK sends
    # its prompt.
    scenario = os.environ.get("STUB_SCENARIO", "plan")
    session = "stub-session"

    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue

        try:
            msg = json.loads(line)
        except json.JSONDecodeError:
            continue

        # Answer the initialize handshake so the SDK proceeds to the prompt.
        if msg.get("type") == "control_request":
            emit({
                "type": "control_response",
                "response": {
                    "subtype": "success",
                    "request_id": msg.get("request_id"),
                    "response": {},
                },
            })
            continue

        if msg.get("type") != "user":
            continue

        replay(scenario, session)

        return


def replay(scenario, session):
    """Emit the message stream for the named scenario."""
    if scenario == "plan":
        plan = {
            "remove": [{"start_line": 4, "end_line": 6}],
            "fold": [],
            "replace": [],
            "summary": "Drops the boilerplate.",
        }
        emit(assistant("```json\n" + json.dumps(plan) + "\n```", session))
        emit(result(session, is_error=False))

        return

    if scenario == "retries":
        # A model that is never reached: the transport retries, no assistant
        # text is ever produced, and the CLI signs off with its own internal
        # diagnostic rather than an explanation.
        for attempt in range(1, 4):
            emit({
                "type": "system",
                "subtype": "api_retry",
                "attempt": attempt,
                "max_retries": 3,
                "retry_delay_ms": 1,
                "error_status": 401,
                "error": "authentication_error",
                "uuid": f"retry-{attempt}",
                "session_id": session,
            })
        emit(result(session, is_error=True, result_text="",
                    errors=["[ede_diagnostic] result_type=user"]))

        return

    if scenario == "hooks":
        # A subprocess that loaded external automation: hook events fire and
        # consume the turn before the model answers.
        for event in ("SessionStart", "Stop"):
            emit({
                "type": "system",
                "subtype": "hook_started",
                "hook_id": f"hook-{event}",
                "hook_name": f"{event}Hook",
                "hook_event": event,
                "uuid": f"hook-{event}",
                "session_id": session,
            })
        emit(result(session, is_error=True, result_text=""))

        return

    if scenario == "denials":
        # A model looping on a tool the policy will never allow.
        for i in range(4):
            emit({
                "type": "system",
                "subtype": "permission_denied",
                "tool_name": "Read",
                "tool_use_id": f"tu-{i}",
                "message": "no repository is configured",
                "uuid": f"deny-{i}",
                "session_id": session,
            })
        emit(result(session, is_error=True, result_text=""))

        return

    # Unknown scenario: say nothing, which is itself a case worth testing.
    emit(result(session, is_error=False))


def assistant(text, session):
    """Build an assistant message carrying a single text block."""
    return {
        "type": "assistant",
        "uuid": "assistant-1",
        "session_id": session,
        "message": {
            "role": "assistant",
            "content": [{"type": "text", "text": text}],
        },
    }


def result(session, is_error, result_text="done", errors=None):
    """Build the terminal result message."""
    return {
        "type": "result",
        "subtype": "error_during_execution" if is_error else "success",
        "status": "error" if is_error else "success",
        "is_error": is_error,
        "result": result_text,
        "errors": errors or [],
        "duration_ms": 1,
        "num_turns": 1,
        "total_cost_usd": 0,
        "uuid": "result-1",
        "session_id": session,
    }


if __name__ == "__main__":
    main()
