# AI Usage

I use AI-assisted development tools including Cursor, Zoekt, Code Graph, and custom AI skills.

## Tools

* **Cursor** — AI-assisted coding, debugging, refactoring, code review, and implementation.
* **Zoekt** — fast code search to help AI and myself find relevant code and understand the codebase.
* **Code Graph** — understanding relationships and dependencies between files, functions, classes, and components.
* **Skills** — reusable instructions and workflows for specific development tasks.

## Development Workflow

I use AI throughout the development process:

1. **Codebase exploration** — Zoekt and Code Graph help locate relevant code and understand dependencies.
2. **Planning** — AI analyzes the requirements and suggests implementation approaches.
3. **Implementation** — AI assists with writing and modifying code.
4. **Testing & debugging** — AI helps identify edge cases, write tests, and investigate errors.
5. **Code review** — AI reviews the final changes for bugs, edge cases, and unnecessary complexity.

## Prompting

I provide the AI with the relevant task, code, constraints, and expected behavior. I prefer focused prompts and incremental context instead of providing the entire codebase.

Example:

```text
Analyze this task and the relevant code.

Explain:
- what needs to change
- which files are involved
- possible edge cases
- the simplest implementation approach

Do not modify unrelated code.
```

AI-generated results are always reviewed against the actual code, requirements, tests, and runtime behavior. Incorrect assumptions or hallucinated APIs are corrected by providing additional context and constraints.

## Token Usage

I manage token usage by keeping prompts focused, using code search and Code Graph to provide only relevant context, and using reusable skills instead of repeating large instructions.
