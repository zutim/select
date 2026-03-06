# 完整策略：次低点+骑线+金叉买入 + 1分钟级移动止盈/止损
import talib
import numpy as np

# ------------------- 工具函数 -------------------
def is_st(stock_code, context):
    df = get_extras('is_st', stock_code, count=1, end_date=context.previous_date)
    return df.iloc[-1, 0] if not df.empty else False

def is_paused(stock_code, context):
    df = get_price(stock_code, end_date=context.previous_date, count=1,
                   frequency='daily', fields='volume')
    return df.iloc[-1]['volume'] == 0

# ------------------- 初始化 -------------------
def initialize(context):
    g.security = get_index_stocks('000300.XSHG')  # 可改全A
    g.N = 20                                    # 最大持仓
    # 1. 盘中止损：09:30 起每 1 分钟跑一次
    # run_interval(check_stop, 1, time='open')
    # 2. 买入扫描：仅 09:35 执行一次
    run_daily(trade_open, time='09:35')

def handle_data(context, data):
    """回测每 1 分钟触发一次"""
    for stock in list(context.portfolio.positions.keys()):
        pos = context.portfolio.positions[stock]
        # 取当前 1 分钟收盘价
        last_close = data[stock].close if stock in data else pos.price
        high_since = get_price(stock, start_date=pos.init_time,
                               end_date=context.current_dt,
                               frequency='1m', fields='high')['high'].max()
        up_rate  = (high_since - pos.avg_cost) / pos.avg_cost
        drawdown = (high_since - last_close) / high_since

        if up_rate >= 0.20 or drawdown >= 0.05:
            order_target(stock, 0)

# =================== 1. 盘中止损（1 分钟） ===================
def check_stop(context):
    for stock in list(context.portfolio.positions.keys()):
        pos = context.portfolio.positions[stock]
        # 取最新 1 分钟收盘价（实时）
        df = get_price(stock, count=1, end_date=context.current_dt,
                       frequency='1m', fields=['close','high'])
        if df.empty:
            continue
        last_close = df['close'].iloc[-1]
        # 自建仓以来最高价（1 分钟级更精细）
        high_since = get_price(stock, start_date=pos.init_time,
                               end_date=context.current_dt,
                               frequency='1m', fields='high')['high'].max()
        up_rate  = (high_since - pos.avg_cost) / pos.avg_cost
        drawdown = (high_since - last_close) / high_since

        if up_rate >= 0.20 or drawdown >= 0.05:
            order_target(stock, 0)

# =================== 2. 买入扫描（09:35 一次） ===================
def trade_open(context):
    buy_list = []
    for stock in g.security:
        if stock in context.portfolio.positions or is_st(stock, context) or is_paused(stock, context):
            continue

        df = attribute_history(stock, 70, '1d', ['close', 'low'], skip_paused=True)
        if len(df) < 65:
            continue
        close = df['close'].values
        low   = df['low'].values
        ma5   = talib.SMA(close, 5)
        ma60  = talib.SMA(close, 60)

        # 1. 10 日内次低点（不含今天）
        ten_low = np.min(low[-11:-1])
        cond1 = (low[-11:-1].min() == ten_low) and (low[-1] > ten_low)
        # 2. 最近 5 日收盘全部站上 5 日线
        cond2 = np.all(close[-5:] > ma5[-5:])
        # 3. 5 日上穿 60 日线（金叉）
        cond3 = (ma5[-2] < ma60[-2]) and (ma5[-1] > ma60[-1])

        if cond1 and cond2 and cond3:
            buy_list.append(stock)
            if len(buy_list) >= g.N:
                break

    # 等权买入
    if buy_list:
        cash_per_stock = context.portfolio.available_cash / len(buy_list)
        for stock in buy_list:
            order_value(stock, cash_per_stock)