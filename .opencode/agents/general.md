---
description: General-purpose agent for researching complex questions and executing multi-step tasks.
mode: subagent
model: deepseek/deepseek-v4-flash
temperature: 0.1
permission:
  edit: allow
  bash:
    "*": allow
  read: allow
  write: allow
  glob: allow
  grep: allow
  task: deny
---

You are a general-purpose agent. You have access to all tools. Complete the task given to you.
