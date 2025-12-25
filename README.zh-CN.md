[English](README.md) | [简体中文](README.zh-CN.md)

# 🧠 TimeLayer CLI

> 一个支持长期思考与记忆积累的本地优先 AI 命令行工具。

TimeLayer CLI 是一个 **local-first（本地优先）** 的个人 AI 系统。它的目标不是短暂的一问一答，而是：

* 长期使用
* 持续记录
* 反思与总结
* 构建一个真正「会积累」的个人记忆系统

> 如果 ChatGPT 会忘记你，这个不会。

---

## 1️⃣ 为什么要做 TimeLayer CLI？

当下大多数 AI 工具都存在一些天然局限：

* 强依赖云端
* 没有长期状态
* 以会话为单位
* 数据与行为不可审计

这些工具非常适合 **短期对话**，但并不适合：

* 长期学习
* 持续写作
* 技术知识积累
* 个人知识沉淀

TimeLayer CLI 的目标很简单：

> **让 AI 具备连续性，而不是每天从零开始。**

---

## 2️⃣ 这不是一个聊天机器人

**这不是一个聊天机器人。**
**这是一个长期运行的个人记忆系统。**

聊天只是输入方式之一，而不是系统的本质。

系统真正关注的是：

* 你长期在关心什么
* 你反复在思考什么
* 哪些问题一再出现
* 你的理解与认知如何随时间演化

---

## 3️⃣ 功能特性

### 🗨️ 上下文对话（`/chat`）

* 面向短期对话流
* 使用滑动上下文窗口
* 不保证历史完整性

### 🧠 记忆问答（`/ask`）

* 面向长期记忆系统
* 使用 **你自己的历史数据** 回答问题
* 基于语义搜索，而非关键词匹配

### 💾 持久化记忆系统

* 原始日志采用 JSONL，不可变（事实层）
* 结构化长期记忆存储于 SQLite
* 支持 日 / 周 / 月 多级抽象总结

### 🔍 语义搜索

* 本地 Embedding
* SQLite 向量索引
* 可随时从日志全量重建

### 🔐 完全本地 & 隐私优先

* 无云端调用
* 无遥测
* 无账号体系
* 所有数据完全由你掌控

---

## 4️⃣ 架构概览

```
用户
 │
 ▼
CLI（Go）
 │
 ├─ /chat   短期对话流
 ├─ /ask    长期记忆问答
 ├─ 日志记录 / 反思调度
 │
 ├───────────────┐
 │               │
 ▼               ▼
llama.cpp        Ollama
（文本生成）     （Embedding / 理解）
 │               │
 ▼               ▼
GGUF 模型        向量表示
（本地）          │
                  ▼
            SQLite（memory.sqlite）
            ├─ 向量索引
            ├─ 事实摘要
            └─ 长期用户特征
```

> llama.cpp 负责「生成」，Ollama 负责「理解与检索」，CLI 负责编排时间与记忆。

---

## 5️⃣ 项目结构（源码）

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

## 6️⃣ ⚙️ 运行环境与依赖（必需）

### 支持平台

* macOS（Intel / Apple Silicon）
* Linux（x86_64 / ARM64）

> Windows 仅支持通过 WSL2 使用。

### Go

* Go **1.21+**

```bash
go version
```

### llama.cpp（文本生成）

```bash
brew install llama.cpp
llama-cli --version
```

### Ollama（Embedding —— **必需**）

> 当前版本 **强依赖 Ollama 提供本地 Embedding 服务**，用于：
>
> * 语义搜索
> * `/ask` 长期记忆检索
> * 长期记忆向量化
>
> **如果 Ollama 未运行，记忆系统将无法工作。**

```bash
brew install ollama
ollama serve
ollama pull nomic-embed-text
```

### GGUF 模型

```text
models/
├── qwen2.5-7b-instruct-q5_k_m-00001-of-00002.gguf
├── qwen2.5-7b-instruct-q5_k_m-00002-of-00002.gguf
```

### 目录初始化

```bash
mkdir -p ~/local-ai/{logs,memory,models,prompts}
```

### 启动 llama-server（推荐）

```bash
llama-server \
  -m models/qwen2.5-7b-instruct-q5_k_m-00001-of-00002.gguf \
  --port 8080
```

---

## 7️⃣ 本地数据与记忆结构（真实运行状态）

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

### `logs/` —— 不可变事实层

* JSONL 形式的时间序列日志，只追加不修改
* `*.daily.json` 为当日反思与抽象结果

### `memory/` —— 长期记忆层

* SQLite 数据库
* 可随时从 logs 全量重建

### `prompts/` —— 稳定认知模板

* daily / weekly / monthly 各自独立
* Prompt 是系统行为的一部分，应视为代码

---

## 8️⃣ 使用说明

启动程序后，TimeLayer CLI 会进入一个 **长期运行的交互式 CLI 环境**（类似 REPL）。

### 基本交互

* 直接输入文本并回车即可对话
* 每一次输入与输出都会被自动记录到不可变日志中

### 内置命令

* `/chat`
  使用短期滑动上下文进行普通对话。

* `/ask <问题>`
  面向 **长期记忆系统** 提问。系统会对历史数据进行语义搜索，并将最相关的记忆注入后再生成回答。

* `/daily`
  手动触发当天的反思与抽象（通常会自动执行）。

* `/weekly`
  生成或更新当前周的总结。

* `/monthly`
  生成或更新当前月的抽象总结。

* `/remember <fact>`
  显式地向系统教授一条**已确认的事实**。该事实会被写入不可变的原始日志，并在每日抽象阶段持久化，之后可通过 `/ask` 被稳定检索和使用。

* `/forget <fact>`
  显式地撤回一条此前记住的事实。该操作不会删除任何历史记录，而是记录一次**认知层面的撤回声明**，在后续推理中覆盖先前的事实认知。



* `/exit` 或 `Ctrl+C`
  安全退出程序。

> 普通聊天并不会自动成为长期记忆。
> 只有被记录并经过抽象处理的内容，才会参与 `/ask` 的语义检索。

---

## 9️⃣ 快速开始

```bash
go run ./cmd/local-ai/main.go
```

---

## 🧭 为什么 TimeLayer 选择 GPLv3

TimeLayer 的核心理念很简单，但也很坚定：  
**一个用于长期思考与记忆沉淀的软件，本身也应当保持自由与透明。**

我们选择 GNU General Public License v3.0（GPLv3），是为了确保：

- 任何人都可以自由地使用、学习、修改和分发 TimeLayer。
- 所有改进和衍生作品都能持续回馈社区，而不是被封闭。
- 没有人可以将 TimeLayer 的核心思想拿走，闭源后变成私有产品。

选择 GPLv3 并不是为了限制使用，  
而是为了保护自由——使用者的自由、贡献者的自由，以及未来维护者的自由。

TimeLayer 被设计为一个长期存在的系统，  
而 GPLv3 能确保这种长期价值不会被攫取并封存。


## 📜 许可证

本项目基于 **GNU GPL v3.0** 协议开源。  
任何基于本项目的修改、再发布或分发，**都必须同样开源并提供完整源码**。

See the [LICENSE](LICENSE) file for details.


---

**如果你在寻找一个可以陪你很多年的个人 AI，这个项目正是为此而设计的。**
