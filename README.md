[English](README.md) | [简体中文](README.zh-CN.md)

# 🧠 TimeLayer CLI

> A local-first AI CLI designed for long-term thinking and memory accumulation.

TimeLayer CLI is a **local-first personal AI system**. Its goal is not short-lived Q&A sessions, but:

* Long-term usage
* Continuous recording
* Reflection and summarization
* Building a truly cumulative personal memory system

> If ChatGPT forgets you — this won’t.

---

## 1️⃣ Why TimeLayer CLI?

Most existing AI tools have inherent limitations:

* Strong dependency on cloud services
* No long-term state
* Session-based interaction
* Data and behavior are not auditable

These tools are excellent for **short conversations**, but poorly suited for:

* Long-term learning
* Continuous writing
* Technical knowledge accumulation
* Personal knowledge archiving

The goal of TimeLayer CLI is simple:

> **Give AI continuity, instead of starting from zero every day.**

---

## 2️⃣ This Is Not a Chatbot

**This is not a chatbot.**
**It is a long-running personal memory system.**

Chat is only one input method — not the essence of the system.

The system focuses on:

* What you care about over time
* What you repeatedly think about
* Questions that keep reappearing
* How your understanding evolves

---

## 3️⃣ Features

### 🗨️ Contextual Chat (`/chat`)

* Short-term conversational flow
* Sliding context window
* No guarantee of historical completeness

### 🧠 Memory Q&A (`/ask`)

* Oriented toward long-term memory
* Answers questions using *your own historical data*
* Semantic search instead of keyword matching

### 💾 Persistent Memory System

* Immutable raw logs in JSONL (fact layer)
* Structured long-term memory stored in SQLite
* Daily / Weekly / Monthly multi-level abstraction

### 🔍 Semantic Search

* Local embeddings
* SQLite-based vector index
* Can be fully rebuilt from logs at any time

### 🔐 Fully Local & Privacy-First

* No cloud calls
* No telemetry
* No accounts
* All data stays under your control

---

## 4️⃣ Architecture Overview

```
User
 │
 ▼
CLI (Go)
 │
 ├─ /chat   Short-term dialogue
 ├─ /ask    Long-term memory Q&A
 ├─ Logging & reflection scheduling
 │
 ├───────────────┐
 │               │
 ▼               ▼
llama.cpp        Ollama
(Text generation) (Embeddings)
 │               │
 ▼               ▼
GGUF models      Vector representations
(Local)           │
                  ▼
            SQLite (memory.sqlite)
            ├─ Vector index
            ├─ Fact summaries
            └─ Long-term user traits
```

> llama.cpp is responsible for *generation*, Ollama for *understanding and retrieval*, and the CLI orchestrates time and memory.

---

## 5️⃣ Source Tree

```
.
├── README.md
├── README.zh-CN.md
├── cmd/
│   └── local-ai/
│       └── main.go
├── internal/
│   └── app/
│       ├── run.go
│       ├── config.go
│       ├── llm.go
│       ├── chat*.go
│       ├── ask.go
│       ├── search.go
│       ├── logger.go
│       ├── db.go
│       ├── index_text.go
│       ├── summary_daily.go
│       ├── summary_weekly.go
│       ├── summary_monthly.go
│       ├── user_fact.go
│       └── tts.go
```

---

## 6️⃣ ⚙️ Environment & Dependencies (Required)

### Supported Platforms

* macOS (Intel / Apple Silicon)
* Linux (x86_64 / ARM64)

> Windows is supported via WSL2 only.

### Go

* Go **1.21+**

```bash
go version
```

### llama.cpp (Text Generation)

```bash
brew install llama.cpp
llama-cli --version
```

### Ollama (Embeddings — **Required**)

> The current version **requires Ollama for local embedding services**, used for:
>
> * Semantic search
> * `/ask` memory retrieval
> * Long-term memory vectorization
>
> **If Ollama is not running, the memory system will not function.**

```bash
brew install ollama
ollama serve
ollama pull nomic-embed-text
```

### GGUF Models

```text
models/
├── qwen2.5-7b-instruct-q5_k_m-00001-of-00002.gguf
├── qwen2.5-7b-instruct-q5_k_m-00002-of-00002.gguf
```

### Directory Initialization

```bash
mkdir -p ~/local-ai/{logs,memory,models,prompts}
```

### Start llama-server (Recommended)

```bash
llama-server \
  -m models/qwen2.5-7b-instruct-q5_k_m-00001-of-00002.gguf \
  --port 8080
```

---

## 7️⃣ Local Data & Memory Layout (Real Runtime State)

```
.
├── logs
│   ├── 2025-12-24.daily.json
│   ├── 2025-12-24.jsonl
│   └── archive
├── memory
│   └── memory.sqlite
├── models
│   ├── qwen2.5-7b-instruct-q5_k_m-00001-of-00002.gguf
│   └── qwen2.5-7b-instruct-q5_k_m-00002-of-00002.gguf
└── prompts
    ├── daily.txt
    ├── weekly.txt
    └── monthly.txt
```

### `logs/` — Immutable Fact Layer

* JSONL append-only timeline of all interactions
* `*.daily.json` contains daily reflective abstractions

### `memory/` — Long-Term Memory Layer

* SQLite database
* Can be fully rebuilt from logs at any time

### `prompts/` — Stable Cognitive Templates

* Separate prompts for daily / weekly / monthly reflection
* Prompts are part of system behavior and should be treated as code

---

## 8️⃣ Usage Guide

After starting the program, TimeLayer CLI enters an interactive REPL-like interface.

### Basic Interaction

* Type plain text and press Enter to chat with the model
* Each input/output is automatically recorded into the immutable log

### Commands

* `/chat`
  Continue normal conversational interaction using short-term sliding context.

* `/ask <question>`
  Ask questions against your **long-term memory**. The system performs semantic search over historical data and injects the most relevant memories before generation.

* `/daily`
  Trigger daily reflection and abstraction manually (normally auto-triggered).

* `/weekly`
  Generate or update the current weekly summary.

* `/monthly`
  Generate or update the current monthly abstraction.

* `/remember <fact>`
  Explicitly teach the system a confirmed fact. The fact will be written into the immutable log and persisted through daily abstraction, making it retrievable via `/ask`.

* `/forget <fact>`
  Explicitly retract a previously remembered fact. This does not delete history, but records a cognitive retraction that will override the earlier fact in future reasoning.

* `/exit` or `Ctrl+C`
  Exit the program safely.

> Normal chat does not automatically become memory. Only logged and abstracted content participates in long-term retrieval.

---

## 9️⃣ Quick Start

```bash
go run ./cmd/local-ai/main.go
```

---

## 🧭 Why TimeLayer Uses GPLv3

TimeLayer is built around a simple but firm belief:  
**software that preserves long-term thinking and memory should itself remain free and transparent.**

We choose the GNU General Public License v3.0 (GPLv3) to ensure that:

- Everyone is free to use, study, modify, and redistribute TimeLayer.
- Improvements and derivative works remain open and benefit the community.
- No one can take the core ideas of TimeLayer, close the source, and turn them into a proprietary product.

GPLv3 is not chosen to restrict usage, but to protect freedom —  
the freedom of users, contributors, and future maintainers.

TimeLayer is designed to be a long-term system.  
GPLv3 helps ensure that this long-term value cannot be extracted and locked away.


## 📜 License

This project is licensed under the **GNU General Public License v3.0 (GPLv3)**.

Any derivative work or redistribution must be released under the same license,
with full source code made available.

See the [LICENSE](LICENSE) file for details.


---

**If you are looking for a personal AI that can accompany you for many years, this project is built for exactly that purpose.**
