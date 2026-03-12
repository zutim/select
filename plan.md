# Plan: 量化策略 diyi.py 迁移至 Go 语言实施方案

## 1. 概述

本项目旨在将现有 Python 量化策略 `other/diyi/diyi.py` 的核心逻辑，使用 Go 语言在 `unified_quant_system` 项目中进行重新实现。

迁移将严格遵循 `other/diyi/diyi.md` 中提出的要求，实现一个自动化的、分为**选股、买入、监控卖出**三个阶段的量化交易流程。

**核心技术栈：**

- **编程语言:** Go
- **数据获取:** `tdx_integration` 模块
- **数据持久化:** 本地 SQLite 数据库 (根据 `vendor` 目录推断)
- **任务调度:** `robfig/cron` (根据 `vendor` 目录推断)

## 2. 架构设计

新的量化策略将作为 `unified_quant_system` 的一个核心模块运行，主要改动和新增部分如下：

1.  **股票池管理 (`data_manager/`):** 新增一个每日任务，用于获取最新的沪深300成分股，过滤ST与停牌股后，保存为 `data/csi300_stocks.json` 文件，供选股模块使用。
2.  **选股逻辑 (`selector/`):** 在 `selector/logic.go` 中实现 Python 脚本中的选股逻辑。这包括计算技术指标（MA均线、10日内次低点）和判断买入条件。
3.  **持仓管理 (`portfolio/`):** 创建一个新的 package `portfolio` 用于管理持仓。它将包含一个 `portfolio.go` 文件和一个 `database.go` 文件，负责通过 SQLite 数据库对持仓记录进行增、删、改、查。
4.  **任务调度与执行 (`main.go`):** 在 `main.go` 中，将使用 cron 调度器来编排整个流程的三个主要任务：
    *   每日开盘前：更新股票池。
    *   每日 09:35：执行选股和买入。
    *   盘中每分钟：执行止盈止损检查。

## 3. 数据模型

我们将在本地 SQLite 数据库中创建一个 `positions` 表来存储持仓信息。

**表名:** `positions`

| 字段名      | 类型      | 描述                             |
| :---------- | :-------- | :------------------------------- |
| `id`        | INTEGER   | 主键，自增                       |
| `stock_code`| TEXT      | 股票代码，例如 "sh600000"        |
| `buy_price` | REAL      | 成本价 (avg_cost)                |
| `buy_time`  | DATETIME  | 买入时间 (init_time)             |
| `status`    | TEXT      | 持仓状态 ("open", "closed")      |
| `sell_price`| REAL      | 卖出价 (可空)                    |
| `sell_time` | DATETIME  | 卖出时间 (可空)                  |

## 4. 详细实施步骤

### **阶段一：准备工作 (股票池更新 & 持仓管理)**

1.  **[数据库]** 创建 `portfolio/database.go`，初始化 SQLite 连接，并包含创建 `positions` 表的逻辑。
2.  **[持仓模型]** 在 `portfolio/portfolio.go` 中定义 `Position` 结构体，并提供 `AddPosition`, `ClosePosition`, `GetOpenPositions` 等数据库操作方法。
3.  **[股票池]** 在 `data_manager/` 中创建一个新文件（例如 `stock_pool.go`），实现 `UpdateCSI300Stocks()` 函数。
    *   该函数通过 `tdx_integration` 获取沪深300成分股。
    *   循环遍历，同样通过 `tdx_integration` 检查每只股票的ST和停牌状态。
    *   过滤后，将最终的股票列表序列化为 JSON，并写入 `data/csi300_stocks.json`。

### **阶段二：核心逻辑实现 (选股 & 卖出)**

1.  **[选股]** 修改 `selector/logic.go`，创建 `SelectStocks()` 函数：
    *   读取 `data/csi300_stocks.json` 获取股票池。
    *   从数据库中读取当前持仓，避免重复买入。
    *   循环处理股票池中的每只股票：
        *   调用 `tdx_integration` 获取近70天的日线数据。
        *   **实现技术指标计算**：
            *   `SMA(close, 5)`: 5日简单移动平均线。
            *   `SMA(close, 60)`: 60日简单移动平均线。
            *   `np.min(low[-11:-1])`: 过去11天到昨天这10个交易日的最低价。
        *   **实现三个核心买入条件 (`cond1`, `cond2`, `cond3`) 的判断逻辑。**
    *   函数返回一个包含所有符合条件待买入股票代码的 `[]string` 列表。

2.  **[止盈止损]** 在 `selector/logic.go` 中，创建 `CheckStopConditions()` 函数：
    *   调用 `portfolio.GetOpenPositions()` 获取所有未平仓的股票。
    *   循环处理每只持仓股：
        *   从数据库记录中获取其 `buy_price` 和 `buy_time`。
        *   调用 `tdx_integration` 获取从 `buy_time` 到现在的 **1分钟 K线** 数据。
        *   计算此期间的最高价 `high_since`。
        *   获取最新的1分钟收盘价 `last_close`。
        *   **实现止盈止损条件判断**：
            *   `up_rate = (high_since - pos.avg_cost) / pos.avg_cost`
            *   `drawdown = (high_since - last_close) / high_since`
            *   如果 `up_rate >= 0.20` 或 `drawdown >= 0.05`，则触发卖出。
    *   函数返回一个包含所有需要卖出股票代码的 `[]string` 列表。

### **阶段三：任务调度与执行**

1.  **[主流程]** 修改 `main.go` 文件。
2.  **[买入执行]** 创建一个 `executeBuy(stocks []string)` 函数。
    *   该函数接收 `SelectStocks` 返回的列表。
    *   对于列表中的每只股票，**打印买入信号到控制台** (例如: `[BUY] Stock: xxx, Time: xxx`)。
    *   调用 `portfolio.AddPosition` 将新持仓写入数据库。
3.  **[卖出执行]** 创建一个 `executeSell(stocks []string)` 函数。
    *   该函数接收 `CheckStopConditions` 返回的列表。
    *   对于列表中的每只股票，**打印卖出信号到控制台** (例如: `[SELL] Stock: xxx, Reason: Stop-Loss, Time: xxx`)。
    *   调用 `portfolio.ClosePosition` 更新数据库中的持仓状态。
4.  **[配置Cron]** 在 `main` 函数中，初始化一个 cron 调度器，并注册以下定时任务：
    *   `"0 9 * * 1-5"` (每个工作日 09:00): 执行 `data_manager.UpdateCSI300Stocks()`。
    *   `"35 9 * * 1-5"` (每个工作日 09:35): 执行 `selector.SelectStocks()` 并将结果传入 `executeBuy`。
    *   `"*/1 9-15 * * 1-5"` (每个工作日 09:30-15:00 的每一分钟): 执行 `selector.CheckStopConditions()` 并将结果传入 `executeSell`。

## 5. 后续规划

- **通知系统:** 当前版本仅打印日志，未来可以轻松地在 `executeBuy` 和 `executeSell` 中加入消息推送逻辑（如钉钉、企业微信、Email等）。
- **交易接口:** 当前版本不执行真实交易，未来可以替换 `executeBuy`/`executeSell` 中的打印逻辑为真实的券商交易API调用。
- **Web界面:** 可以将持仓信息、交易日志等通过 `web_ui` 模块进行可视化展示。
