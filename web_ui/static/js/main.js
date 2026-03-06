document.addEventListener('DOMContentLoaded', function() {
    // 检查Chart.js是否已加载
    if (typeof Chart === 'undefined') {
        console.error('Chart.js library is not loaded!');
        // 创建一个简单的警告提示用户
        const warningDiv = document.createElement('div');
        warningDiv.className = 'alert alert-warning';
        warningDiv.innerHTML = '警告：图表库未加载，回测结果的图表功能将不可用。请检查网络连接。';
        document.querySelector('.container').prepend(warningDiv);
    }

    // API endpoints
    const API_BASE = '/api';
    const ENDPOINTS = {
        DATES: `${API_BASE}/dates`,
        RUN_STRATEGY: `${API_BASE}/run_strategy`,
        STATUS: `${API_BASE}/status`,
        BACKTEST: `${API_BASE}/backtest`,
        BACKTEST_METRICS: `${API_BASE}/backtest/metrics`
    };

    // DOM Elements
    const dateSelect = document.getElementById('dateSelect');
    const strategySelect = document.getElementById('strategySelect');
    const runAnalysisBtn = document.getElementById('runAnalysisBtn');
    const refreshBtn = document.getElementById('refreshBtn');
    const exportBtn = document.getElementById('exportBtn');
    const resultsBody = document.getElementById('resultsBody');
    const resultCount = document.getElementById('resultCount');
    const systemStatus = document.getElementById('systemStatus');
    const systemVersion = document.getElementById('systemVersion');
    const lastUpdate = document.getElementById('lastUpdate');
    const availableDatesCount = document.getElementById('availableDatesCount');
    
    // Backtest elements
    const startDateInput = document.getElementById('startDate');
    const endDateInput = document.getElementById('endDate');
    const initialCapitalInput = document.getElementById('initialCapital');
    const backtestStrategySelect = document.getElementById('backtestStrategySelect');
    const runBacktestBtn = document.getElementById('runBacktestBtn');
    const backtestResultsCard = document.getElementById('backtestResultsCard');
    const hideBacktestResultsBtn = document.getElementById('hideBacktestResults');
    
    // Metric elements
    const totalReturnEl = document.getElementById('totalReturn');
    const annualReturnEl = document.getElementById('annualReturn');
    const volatilityEl = document.getElementById('volatility');
    const maxDrawdownEl = document.getElementById('maxDrawdown');
    const sharpeRatioEl = document.getElementById('sharpeRatio');
    const winRateEl = document.getElementById('winRate');
    
    // Chart canvases
    const equityChartCanvas = document.getElementById('equityChart');
    const returnsChartCanvas = document.getElementById('returnsChart');
    
    // Chart instances
    let equityChart = null;
    let returnsChart = null;

    // Initialize the app
    initApp();

    async function initApp() {
        await loadSystemStatus();
        await loadAvailableDates();
        setupEventListeners();
        setupDateInputs();
    }

    // Setup date inputs with default values
    function setupDateInputs() {
        const today = new Date();
        const priorDate = new Date();
        priorDate.setDate(today.getDate() - 30); // 默认30天前
        
        startDateInput.valueAsDate = priorDate;
        endDateInput.valueAsDate = today;
    }

    // Setup event listeners
    function setupEventListeners() {
        runAnalysisBtn.addEventListener('click', runAnalysis);
        runBacktestBtn.addEventListener('click', runBacktest);
        refreshBtn.addEventListener('click', handleRefresh);
        exportBtn.addEventListener('click', exportResults);
        hideBacktestResultsBtn.addEventListener('click', hideBacktestResults);
        
        // Auto-refresh system status every 30 seconds
        setInterval(loadSystemStatus, 30000);
    }

    // Load system status
    async function loadSystemStatus() {
        try {
            const response = await fetch(ENDPOINTS.STATUS);
            const data = await response.json();
            
            if (data.status === 'running') {
                systemStatus.textContent = '运行中';
                systemStatus.className = 'status-running';
            } else {
                systemStatus.textContent = '已停止';
                systemStatus.className = 'status-stopped';
            }
            
            systemVersion.textContent = data.version || '-';
            lastUpdate.textContent = data.last_update || '-';
        } catch (error) {
            console.error('Error loading system status:', error);
            systemStatus.textContent = '错误';
            systemStatus.className = 'status-stopped';
        }
    }

    // Load available dates
    async function loadAvailableDates() {
        try {
            const response = await fetch(ENDPOINTS.DATES);
            const data = await response.json();
            
            dateSelect.innerHTML = '';
            if (data.dates && data.dates.length > 0) {
                data.dates.forEach(date => {
                    const option = document.createElement('option');
                    option.value = date;
                    option.textContent = date;
                    dateSelect.appendChild(option);
                });
                
                // Set the first date as selected
                if (data.dates.length > 0) {
                    dateSelect.value = data.dates[0];
                }
                
                availableDatesCount.textContent = data.dates.length;
            } else {
                const option = document.createElement('option');
                option.textContent = '无可用日期';
                dateSelect.appendChild(option);
                
                availableDatesCount.textContent = '0';
            }
        } catch (error) {
            console.error('Error loading dates:', error);
            dateSelect.innerHTML = '<option>加载失败</option>';
            availableDatesCount.textContent = '0';
        }
    }

    // Run analysis
    async function runAnalysis() {
        const selectedDate = dateSelect.value;
        const selectedStrategy = strategySelect.value;

        if (!selectedDate) {
            alert('请选择一个日期');
            return;
        }

        // Show loading state
        runAnalysisBtn.disabled = true;
        runAnalysisBtn.textContent = '分析中...';

        try {
            const response = await fetch(ENDPOINTS.RUN_STRATEGY, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    date: selectedDate,
                    strategy: selectedStrategy
                })
            });

            const data = await response.json();

            if (response.ok) {
                displayResults(data.stocks || []);
                resultCount.textContent = data.count || 0;
            } else {
                throw new Error(data.error || '分析失败');
            }
        } catch (error) {
            console.error('Error running analysis:', error);
            alert(`分析失败: ${error.message}`);
            resultCount.textContent = '0';
        } finally {
            // Reset button state
            runAnalysisBtn.disabled = false;
            runAnalysisBtn.textContent = '运行分析';
        }
    }

    // Run backtest
    async function runBacktest() {
        const startDate = startDateInput.value;
        const endDate = endDateInput.value;
        const initialCapital = parseFloat(initialCapitalInput.value);
        const strategy = backtestStrategySelect.value;

        if (!startDate || !endDate) {
            alert('请选择开始日期和结束日期');
            return;
        }

        if (isNaN(initialCapital) || initialCapital <= 0) {
            alert('请输入有效的初始资金');
            return;
        }

        if (new Date(startDate) > new Date(endDate)) {
            alert('开始日期不能晚于结束日期');
            return;
        }

        // Show loading state
        runBacktestBtn.disabled = true;
        runBacktestBtn.textContent = '回测中...';

        try {
            const response = await fetch(ENDPOINTS.BACKTEST, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    start_date: startDate,
                    end_date: endDate,
                    initial_capital: initialCapital,
                    strategy: strategy
                })
            });

            const data = await response.json();

            if (response.ok) {
                displayBacktestResults(data);
                showBacktestResults();
            } else {
                throw new Error(data.error || '回测失败');
            }
        } catch (error) {
            console.error('Error running backtest:', error);
            alert(`回测失败: ${error.message}`);
        } finally {
            // Reset button state
            runBacktestBtn.disabled = false;
            runBacktestBtn.textContent = '运行回测';
        }
    }

    // Display backtest results
    function displayBacktestResults(data) {
        // Update metric displays
        totalReturnEl.textContent = (data.total_return * 100).toFixed(2) + '%';
        annualReturnEl.textContent = (data.annual_return * 100).toFixed(2) + '%';
        volatilityEl.textContent = (data.volatility * 100).toFixed(2) + '%';
        maxDrawdownEl.textContent = (data.max_drawdown * 100).toFixed(2) + '%';
        sharpeRatioEl.textContent = data.sharpe_ratio.toFixed(2);
        winRateEl.textContent = (data.win_rate * 100).toFixed(2) + '%';

        // Check if Chart.js is loaded
        if (typeof Chart === 'undefined') {
            console.error('Chart.js library is not loaded');
            alert('图表库未加载，请检查网络连接');
            return;
        }

        // Create equity curve chart
        if (equityChart) {
            equityChart.destroy();
        }

        const equityDates = data.equity_curve.map(item => item.date);
        const equityValues = data.equity_curve.map(item => item.value);

        try {
            equityChart = new Chart(equityChartCanvas, {
                type: 'line',
                data: {
                    labels: equityDates,
                    datasets: [{
                        label: '净值曲线',
                        data: equityValues,
                        borderColor: 'rgb(75, 192, 192)',
                        backgroundColor: 'rgba(75, 192, 192, 0.2)',
                        tension: 0.1
                    }]
                },
                options: {
                    responsive: true,
                    plugins: {
                        title: {
                            display: true,
                            text: '策略净值曲线'
                        }
                    },
                    scales: {
                        y: {
                            beginAtZero: false
                        }
                    }
                }
            });
        } catch (chartError) {
            console.error('Error creating equity chart:', chartError);
        }

        // Create returns chart
        if (returnsChart) {
            returnsChart.destroy();
        }

        const returnDates = data.daily_returns.map(item => item.date);
        const returnValues = data.daily_returns.map(item => item.return * 100); // Convert to percentage

        try {
            returnsChart = new Chart(returnsChartCanvas, {
                type: 'bar',
                data: {
                    labels: returnDates,
                    datasets: [{
                        label: '日收益率 (%)',
                        data: returnValues,
                        backgroundColor: returnValues.map(value => value >= 0 ? 'rgba(75, 192, 192, 0.6)' : 'rgba(255, 99, 132, 0.6)')
                    }]
                },
                options: {
                    responsive: true,
                    plugins: {
                        title: {
                            display: true,
                            text: '日收益率分布'
                        }
                    },
                    scales: {
                        y: {
                            beginAtZero: false
                        }
                    }
                }
            });
        } catch (chartError) {
            console.error('Error creating returns chart:', chartError);
        }

        // 创建交易记录表格
        createTransactionsTable(data.transactions);
    }

    // 创建交易记录表格
    function createTransactionsTable(transactions) {
        if (!transactions || transactions.length === 0) {
            return;
        }

        // 创建一个包含交易记录的卡片
        let transactionsCard = document.getElementById('transactionsCard');
        if (!transactionsCard) {
            transactionsCard = document.createElement('div');
            transactionsCard.id = 'transactionsCard';
            transactionsCard.className = 'card mt-4';
            transactionsCard.innerHTML = `
                <div class="card-header d-flex justify-content-between align-items-center">
                    <h5>交易记录</h5>
                </div>
                <div class="card-body">
                    <div class="table-responsive">
                        <table class="table table-striped" id="transactionsTable">
                            <thead>
                                <tr>
                                    <th>日期</th>
                                    <th>代码</th>
                                    <th>名称</th>
                                    <th>类型</th>
                                    <th>数量</th>
                                    <th>价格</th>
                                    <th>金额</th>
                                    <th>手续费</th>
                                </tr>
                            </thead>
                            <tbody id="transactionsBody">
                            </tbody>
                        </table>
                    </div>
                </div>
            `;
            document.querySelector('#backtestResultsCard .card-body').appendChild(transactionsCard);
        }

        const tbody = document.getElementById('transactionsBody');
        tbody.innerHTML = '';

        // 只显示前50条交易记录，避免页面过长
        const displayTransactions = transactions.slice(0, 50);
        
        displayTransactions.forEach(transaction => {
            const row = document.createElement('tr');
            row.innerHTML = `
                <td>${transaction.date}</td>
                <td>${transaction.code}</td>
                <td>${transaction.name}</td>
                <td><span class="${transaction.type === 'BUY' ? 'text-success' : 'text-danger'}">${transaction.type}</span></td>
                <td>${transaction.quantity}</td>
                <td>¥${transaction.price.toFixed(2)}</td>
                <td>¥${transaction.amount.toFixed(2)}</td>
                <td>¥${transaction.commission.toFixed(2)}</td>
            `;
            tbody.appendChild(row);
        });

        // 如果交易记录超过50条，显示提示
        if (transactions.length > 50) {
            const noticeRow = document.createElement('tr');
            noticeRow.innerHTML = `
                <td colspan="8" class="text-center text-muted">
                    ... 还有 ${transactions.length - 50} 条交易记录未显示
                </td>
            `;
            tbody.appendChild(noticeRow);
        }
    }

    // Show backtest results card
    function showBacktestResults() {
        backtestResultsCard.classList.remove('d-none');
    }

    // Hide backtest results card
    function hideBacktestResults() {
        backtestResultsCard.classList.add('d-none');
    }

    // Display results in table
    function displayResults(stocks) {
        resultsBody.innerHTML = '';

        if (!stocks || stocks.length === 0) {
            const row = document.createElement('tr');
            row.innerHTML = `
                <td colspan="4" class="text-center">没有找到符合条件的股票</td>
            `;
            resultsBody.appendChild(row);
            return;
        }

        stocks.forEach(stock => {
            const row = document.createElement('tr');
            row.innerHTML = `
                <td>${stock.code}</td>
                <td>${stock.name}</td>
                <td>${stock.date}</td>
                <td>${stock.strategy}</td>
            `;
            resultsBody.appendChild(row);
        });
    }

    // Handle refresh
    async function handleRefresh() {
        await loadSystemStatus();
        await loadAvailableDates();
        alert('数据已刷新');
    }

    // Export results
    function exportResults() {
        const rows = document.querySelectorAll('#resultsTable tbody tr');
        if (rows.length === 0 || rows[0].textContent.includes('没有找到')) {
            alert('没有数据可导出');
            return;
        }

        let csvContent = '代码,名称,日期,策略\n';
        
        rows.forEach(row => {
            const cells = row.querySelectorAll('td');
            const rowData = Array.from(cells).map(cell => `"${cell.textContent}"`).join(',');
            csvContent += rowData + '\n';
        });

        const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.setAttribute('href', url);
        link.setAttribute('download', `quant_results_${new Date().toISOString().slice(0, 10)}.csv`);
        link.style.visibility = 'hidden';
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
    }
});