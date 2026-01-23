#!/usr/bin/env python3
"""
量化选股和回测功能演示脚本
此脚本演示如何通过HTTP API调用量化选股和回测功能
"""

import requests
import json
import time
from datetime import datetime, timedelta

# API基础地址
BASE_URL = "http://localhost:8080"

def test_strategy_stocks():
    """测试策略选股功能"""
    print("=== 测试策略选股功能 ===")
    try:
        response = requests.get(f"{BASE_URL}/api/strategy/stocks")
        if response.status_code == 200:
            data = response.json()
            if data['code'] == 0:
                result = data['data']
                print(f"选股日期: {result['date']}")
                print(f"符合条件的股票总数: {len(result['qualified_stocks'])}")
                print(f"首板高开股票: {result['sbgk_stocks']}")
                print(f"首板低开股票: {result['sbdk_stocks']}")
                print(f"弱转强股票: {result['rzq_stocks']}")
                print(f"所有符合条件的股票: {result['qualified_stocks']}")
            else:
                print(f"API返回错误: {data['message']}")
        else:
            print(f"请求失败: {response.status_code}")
    except Exception as e:
        print(f"请求异常: {e}")

def test_get_stock_info(code):
    """测试获取股票信息功能"""
    print(f"\n=== 测试获取股票信息功能 (股票: {code}) ===")
    try:
        response = requests.get(f"{BASE_URL}/api/stock-info-extended", params={"code": code})
        if response.status_code == 200:
            data = response.json()
            if data['code'] == 0:
                stock_info = data['data']
                print(json.dumps(stock_info, indent=2, ensure_ascii=False))
            else:
                print(f"API返回错误: {data['message']}")
        else:
            print(f"请求失败: {response.status_code}")
    except Exception as e:
        print(f"请求异常: {e}")

def test_get_market_cap(code):
    """测试获取市值数据功能"""
    print(f"\n=== 测试获取市值数据功能 (股票: {code}) ===")
    try:
        response = requests.get(f"{BASE_URL}/api/market-cap", params={"code": code})
        if response.status_code == 200:
            data = response.json()
            if data['code'] == 0:
                market_cap = data['data']
                print(json.dumps(market_cap, indent=2, ensure_ascii=False))
            else:
                print(f"API返回错误: {data['message']}")
        else:
            print(f"请求失败: {response.status_code}")
    except Exception as e:
        print(f"请求异常: {e}")

def test_backtest(start_date, end_date):
    """测试回测功能"""
    print(f"\n=== 测试回测功能 ({start_date} 至 {end_date}) ===")
    try:
        params = {
            "start_date": start_date,
            "end_date": end_date
        }
        response = requests.get(f"{BASE_URL}/api/backtest", params=params)
        if response.status_code == 200:
            data = response.json()
            if data['code'] == 0:
                backtest_result = data['data']
                print(f"回测结果数量: {backtest_result['count']}")
                print(f"回测详情: {json.dumps(backtest_result['list'][:2], indent=2, ensure_ascii=False)}")  # 只显示前2个结果
            else:
                print(f"API返回错误: {data['message']}")
        else:
            print(f"请求失败: {response.status_code}")
    except Exception as e:
        print(f"请求异常: {e}")

def main():
    print("量化选股和回测功能API测试")
    print("="*50)
    
    # 确保服务已启动
    try:
        health_response = requests.get(f"{BASE_URL}/api/health")
        if health_response.status_code == 200:
            print("服务健康检查通过")
        else:
            print("服务可能未启动，请先启动服务")
            return
    except:
        print("服务可能未启动，请先启动服务")
        return
    
    # 测试策略选股
    test_strategy_stocks()
    
    # 获取一些股票代码用于后续测试
    test_stocks = []
    try:
        strategy_response = requests.get(f"{BASE_URL}/api/strategy/stocks")
        if strategy_response.status_code == 200:
            data = strategy_response.json()
            if data['code'] == 0 and data['data']['qualified_stocks']:
                test_stocks = data['data']['qualified_stocks'][:2]  # 取前2个股票用于测试
    except:
        pass
    
    # 如果没有获取到符合条件的股票，使用一些常见的股票代码
    if not test_stocks:
        test_stocks = ['000001', '600000']  # 示例股票代码
    
    # 测试获取股票信息
    for stock_code in test_stocks[:1]:  # 只测试第一个
        test_get_stock_info(stock_code)
        time.sleep(0.5)  # 稍微延时避免请求过快
    
    # 测试获取市值数据
    for stock_code in test_stocks[:1]:  # 只测试第一个
        test_get_market_cap(stock_code)
        time.sleep(0.5)  # 稍微延时避免请求过快
    
    # 测试回测功能（使用最近一周的数据）
    end_date = datetime.now().strftime('%Y-%m-%d')
    start_date = (datetime.now() - timedelta(days=7)).strftime('%Y-%m-%d')
    test_backtest(start_date, end_date)

if __name__ == "__main__":
    main()