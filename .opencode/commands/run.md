---
description: Start the application
agent: runner
model: opencode-go/mimo-v2.5
---

Start the application using the project's configured run command.

Rules:
- Only execute the run command.
- Do not modify any files.
- Do not install anything.
- Do not fix errors.
- Do not run tests.
- Do not inspect or change the project unless necessary to execute the command.
- If the process starts successfully, keep it running and report that it started.
- If it fails immediately, report the error and stop.