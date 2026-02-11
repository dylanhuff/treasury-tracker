import { useState, useEffect } from 'react';
import { LineChart, Card, Title, Select, SelectItem } from '@tremor/react';
import { fetchHistoricalYields } from '../services/api';
import type { HistoricalYieldData } from '../services/api';

type SelectableTerm = "2Y" | "5Y" | "10Y";

export default function YieldCurve() {
  const [selectedPeriod, setSelectedPeriod] = useState<string>('3M');
  const [historicalData, setHistoricalData] = useState<HistoricalYieldData | null>(null);
  const [selectedTerms, setSelectedTerms] = useState<SelectableTerm[]>(['10Y', '5Y', '2Y']);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  // Fetch historical yields when component mounts or selectedPeriod changes
  useEffect(() => {
    const loadHistoricalYields = async () => {
      try {
        setLoading(true);
        setError(null);
        const data = await fetchHistoricalYields(selectedPeriod);
        setHistoricalData(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to fetch historical yields');
      } finally {
        setLoading(false);
      }
    };

    loadHistoricalYields();
  }, [selectedPeriod]); // Re-fetch when period changes

  const handleRetry = () => {
    setHistoricalData(null);
    setError(null);
    setLoading(true);

    fetchHistoricalYields(selectedPeriod)
      .then(data => setHistoricalData(data))
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to fetch historical yields'))
      .finally(() => setLoading(false));
  };

  const formatDateShort = (dateStr: string): string => {
    const date = new Date(dateStr + 'T00:00:00');
    const month = (date.getMonth() + 1).toString();
    const day = date.getDate().toString();
    const year = date.getFullYear().toString().slice(-2);
    return `${month}/${day}/${year}`;
  };

  const handleTermToggle = (term: SelectableTerm) => {
    setSelectedTerms(prev => {
      if (prev.includes(term)) {
        if (prev.length === 1) return prev;
        return prev.filter(t => t !== term);
      } else {
        return [...prev, term];
      }
    });
  };

  // Transform data: add formatted date field for cleaner x-axis
  const rawChartData = historicalData?.data || [];
  const chartData = rawChartData.map(point => ({
    ...point,
    displayDate: formatDateShort(point.date as string)
  }));

  const calculateYAxisRange = () => {
    if (chartData.length === 0 || selectedTerms.length === 0) {
      return { minValue: 0, maxValue: 8 };
    }

    let min = Infinity;
    let max = -Infinity;

    chartData.forEach(point => {
      selectedTerms.forEach(term => {
        const value = point[term];
        if (typeof value === 'number') {
          min = Math.min(min, value);
          max = Math.max(max, value);
        }
      });
    });

    // Add small padding
    const padding = (max - min) * 0.1;
    const paddedMin = min - padding;
    const paddedMax = max + padding;

    // Round to nice even intervals (0.25 increments)
    const increment = 0.25;
    const roundedMin = Math.floor(paddedMin / increment) * increment;
    const roundedMax = Math.ceil(paddedMax / increment) * increment;

    return {
      minValue: Math.max(0, Number(roundedMin.toFixed(2))),
      maxValue: Number(roundedMax.toFixed(2))
    };
  };

  const { minValue, maxValue } = calculateYAxisRange();

  // Time period options for dropdown
  const periodOptions = [
    { value: '1W', label: '1 Week' },
    { value: '1M', label: '1 Month' },
    { value: '3M', label: '3 Months' },
    { value: '6M', label: '6 Months' },
    { value: '1Y', label: '1 Year' },
    { value: '5Y', label: '5 Years' },
    { value: '10Y', label: '10 Years' },
    { value: '30Y', label: '30 Years' }
  ];

  const availableTerms: { value: SelectableTerm; label: string; fullLabel: string }[] = [
    { value: '2Y', label: '2Y', fullLabel: '2-Year Note' },
    { value: '5Y', label: '5Y', fullLabel: '5-Year Note' },
    { value: '10Y', label: '10Y', fullLabel: '10-Year Note' }
  ];

  // Loading state
  if (loading) {
    return (
      <Card>
        <div className="flex items-center justify-center h-64">
          <p className="text-gray-500 dark:text-gray-400">Loading historical yields...</p>
        </div>
      </Card>
    );
  }

  // Error state
  if (error) {
    return (
      <Card>
        <div className="flex flex-col items-center justify-center h-64 space-y-4">
          <p className="text-red-600 dark:text-red-400">{error}</p>
          <button
            onClick={handleRetry}
            className="px-4 py-2 bg-blue-500 dark:bg-blue-600 text-white rounded hover:bg-blue-600 dark:hover:bg-blue-700 transition-colors"
          >
            Retry
          </button>
        </div>
      </Card>
    );
  }

  // Success state with chart
  return (
    <Card data-tour-id="yield-curve-chart">
      {/* Controls Row: Period Selector + Term Selector */}
      <div className="flex flex-col sm:flex-row gap-4 mb-4 items-start sm:items-center justify-between">
        {/* Time Period Dropdown */}
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-gray-700 dark:text-gray-300 whitespace-nowrap">
            Time Period:
          </span>
          <Select
            value={selectedPeriod}
            onValueChange={setSelectedPeriod}
            className="w-40"
          >
            {periodOptions.map(({ value, label }) => (
              <SelectItem key={value} value={value}>
                {label}
              </SelectItem>
            ))}
          </Select>
        </div>

        {/* Yield Curves Selector - Checkboxes */}
        <div className="flex gap-3 items-center flex-wrap">
          <span className="text-sm font-medium text-gray-700 dark:text-gray-300 whitespace-nowrap">
            Yield Curves:
          </span>
          {availableTerms.map(({ value, label, fullLabel }) => (
            <label
              key={value}
              className="flex items-center gap-1.5 cursor-pointer select-none"
              title={fullLabel}
            >
              <input
                type="checkbox"
                checked={selectedTerms.includes(value)}
                onChange={() => handleTermToggle(value)}
                className="w-4 h-4 rounded border-gray-300 text-blue-500 focus:ring-blue-500 cursor-pointer"
              />
              <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
                {label}
              </span>
            </label>
          ))}
        </div>
      </div>

      {/* Chart Title */}
      <Title className="text-center">Historical Treasury Yields ({selectedPeriod})</Title>

      {/* Historical Trends Chart */}
      <LineChart
        className="mt-4 h-80"
        data={chartData}
        index="displayDate"  // Use formatted dates (MM/DD/YY)
        categories={selectedTerms}  // Dynamic: only show selected terms
        colors={["blue", "emerald", "violet"]}
        valueFormatter={(value) => `${value.toFixed(2)}%`}
        yAxisWidth={56}
        showLegend={true}
        showGridLines={true}
        // Y-axis improvements
        yAxisLabel="Yield (%)"
        minValue={minValue}
        maxValue={maxValue}
        // X-axis improvements
        xAxisLabel="Date"
        startEndOnly={false}  // Show multiple dates
        tickGap={selectedPeriod === '1Y' ? 30 : selectedPeriod === '6M' ? 20 : 10}  // Adaptive tick density
      />
    </Card>
  );
}
