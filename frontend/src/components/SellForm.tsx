import { useState, useEffect } from 'react';
import { Card, Title, Select, SelectItem, Button } from '@tremor/react';
import { useCurrentUser } from '../contexts/CurrentUserContext';
import { useTransactionRefresh } from '../contexts/TransactionRefreshContext';
import { sellTreasury, fetchUserHoldings, type Holding } from '../services/api';
import { formatCurrency, formatNumberInputWithCommas } from '../utils/formatters';

// Helper function to calculate days between two dates
function daysBetween(startDate: Date, endDate: Date): number {
  const diff = endDate.getTime() - startDate.getTime();
  return Math.floor(diff / (1000 * 60 * 60 * 24));
}

// Helper function to get term duration in days
function getTermDays(term: string): number {
  const termMap: Record<string, number> = {
    '1M': 30,
    '3M': 90,
    '6M': 180,
    '1Y': 365,
    '2Y': 730,
    '5Y': 1825,
    '10Y': 3650,
    '30Y': 10950,
  };
  return termMap[term] || 365;
}

// Helper function to format holding for dropdown
function formatHolding(holding: Holding): string {
  const purchaseDate = new Date(holding.purchase_date).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  });
  return `${holding.term} @ ${parseFloat(holding.yield_at_purchase).toFixed(2)}% - ${formatCurrency(holding.remaining_amount, 0)} (${purchaseDate})`;
}

export function SellForm() {
  const { currentUser, updateUser } = useCurrentUser();
  const { refreshTransactions } = useTransactionRefresh();

  const [holdings, setHoldings] = useState<Holding[]>([]);
  const [selectedHoldingId, setSelectedHoldingId] = useState<string>('');
  const [amount, setAmount] = useState<number | undefined>(undefined);
  const [displayValue, setDisplayValue] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [loadingHoldings, setLoadingHoldings] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<boolean>(false);

  // Get selected holding object
  const selectedHolding = holdings.find(h => h.id.toString() === selectedHoldingId);

  // Load holdings when user changes
  useEffect(() => {
    const loadHoldings = async () => {
      if (!currentUser) {
        setHoldings([]);
        setSelectedHoldingId('');
        return;
      }

      setLoadingHoldings(true);
      try {
        const userHoldings = await fetchUserHoldings(currentUser.id);
        setHoldings(userHoldings);

        if (selectedHoldingId) {
          const stillExists = userHoldings.find(h => h.id.toString() === selectedHoldingId);
          if (!stillExists) {
            setSelectedHoldingId('');
            setAmount(undefined);
            setDisplayValue('');
          }
        }
      } catch {
        setHoldings([]);
      } finally {
        setLoadingHoldings(false);
      }
    };

    loadHoldings();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentUser]); // Only refetch when user changes, not when selecting different holdings

  // Calculate projected proceeds
  const calculateProceeds = (): { principal: number; yield: number; total: number } | null => {
    if (!selectedHolding || !amount || amount <= 0) {
      return null;
    }

    const principal = amount;
    const yieldRate = parseFloat(selectedHolding.yield_at_purchase);
    const purchaseDate = new Date(selectedHolding.purchase_date);
    const currentDate = new Date();
    const daysHeld = daysBetween(purchaseDate, currentDate);
    const termDays = getTermDays(selectedHolding.term);
    const proportionHeld = Math.min(daysHeld / termDays, 1.0);
    const yieldEarned = principal * (yieldRate / 100) * proportionHeld;
    const totalProceeds = principal + yieldEarned;

    return {
      principal,
      yield: yieldEarned,
      total: totalProceeds,
    };
  };

  const proceeds = calculateProceeds();

  const handleAmountChange = (value: string) => {
    let cleanValue = value.replace(/[^\d.]/g, '');
    const parts = cleanValue.split('.');

    if (parts.length > 2) {
      cleanValue = parts[0] + '.' + parts.slice(1).join('');
    }

    if (parts.length === 2 && parts[1].length > 2) {
      cleanValue = parts[0] + '.' + parts[1].substring(0, 2);
    }

    if (cleanValue === '') {
      setAmount(undefined);
      setDisplayValue('');
      return;
    }

    if (cleanValue === '.') {
      setAmount(undefined);
      setDisplayValue('0.');
      return;
    }

    const formattedValue = formatNumberInputWithCommas(cleanValue);
    setDisplayValue(formattedValue);

    const parsedValue = parseFloat(cleanValue);
    if (!isNaN(parsedValue)) {
      setAmount(parsedValue);
    }
  };

  const validateForm = (): string | null => {
    if (!selectedHolding) {
      return 'Please select a holding to sell';
    }

    if (!amount || amount <= 0) {
      return 'Please enter an amount greater than $0';
    }

    if (!currentUser) {
      return 'No user selected';
    }

    const remainingAmount = parseFloat(selectedHolding.remaining_amount);
    if (amount > remainingAmount) {
      return `Insufficient remaining amount. Available: ${formatCurrency(remainingAmount, 0)}`;
    }

    return null; // Valid
  };

  const handleSell = async () => {
    if (!currentUser || !selectedHolding) return;

    // Clear previous state
    setError(null);
    setSuccess(false);

    // Validate form
    const validationError = validateForm();
    if (validationError) {
      setError(validationError);
      return;
    }

    try {
      setLoading(true);

      // Call API (amount is guaranteed to be defined by validation)
      const updatedUser = await sellTreasury(
        currentUser.id,
        selectedHolding.id,
        amount!
      );

      // Update user context (balance updates in header)
      updateUser(updatedUser);

      // Refresh transaction history
      await refreshTransactions();

      // Reload holdings (useEffect will clear selected holding if fully sold)
      const userHoldings = await fetchUserHoldings(currentUser.id);
      setHoldings(userHoldings);

      // Clear amount fields only (preserve selected holding for sequential sells)
      setAmount(undefined);
      setDisplayValue('');

      // Show success message
      setSuccess(true);

      // Auto-hide success message after 3 seconds
      setTimeout(() => setSuccess(false), 3000);

    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to sell treasury');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card className="mt-6" data-tour-id="sell-form">
      <Title>Sell Treasury</Title>

      {/* Holdings Dropdown */}
      <div className="mt-4">
        <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
          Select Holding
        </label>
        {loadingHoldings ? (
          <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">Loading holdings...</p>
        ) : holdings.length === 0 ? (
          <p className="mt-2 text-sm text-gray-500 dark:text-gray-400">
            No holdings available to sell. Purchase treasuries first.
          </p>
        ) : (
          <Select
            value={selectedHoldingId}
            onValueChange={setSelectedHoldingId}
            placeholder="Select holding..."
            className="mt-1"
          >
            {holdings.map((holding) => (
              <SelectItem key={holding.id} value={holding.id.toString()}>
                {formatHolding(holding)}
              </SelectItem>
            ))}
          </Select>
        )}

        {/* Available Amount Display */}
        {selectedHolding && (
          <div className="mt-2">
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Available to sell:{' '}
              <span className="font-semibold">
                {formatCurrency(selectedHolding.remaining_amount, 0)}
              </span>
            </p>
          </div>
        )}
      </div>

      {/* Amount Input - Only show when holding selected */}
      {selectedHolding && (
        <div className="mt-4">
          <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
            Amount to Sell
          </label>
          <div className="relative mt-1">
            <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
              <span className="text-gray-500 dark:text-gray-400 text-base">$</span>
            </div>
            <input
              type="text"
              value={displayValue}
              onChange={(e) => handleAmountChange(e.target.value)}
              placeholder="Enter amount"
              className="pl-7 pr-4 py-2 w-full text-base border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500"
            />
          </div>
        </div>
      )}

      {/* Projected Proceeds - Only show when amount > 0 */}
      {proceeds && amount && amount > 0 && (
        <div className="mt-4 p-4 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md">
          <p className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
            Projected Proceeds:
          </p>
          <div className="space-y-1">
            <div className="flex justify-between text-sm">
              <span className="text-gray-600 dark:text-gray-400">Principal:</span>
              <span className="font-semibold text-gray-900 dark:text-gray-100">
                {formatCurrency(proceeds.principal)}
              </span>
            </div>
            <div className="flex justify-between text-sm">
              <span className="text-gray-600 dark:text-gray-400">Yield Earned:</span>
              <span className="font-semibold text-green-600 dark:text-green-400">
                {formatCurrency(proceeds.yield)}
              </span>
            </div>
            <div className="flex justify-between text-sm pt-2 border-t border-gray-300 dark:border-gray-600">
              <span className="font-medium text-gray-700 dark:text-gray-300">Total:</span>
              <span className="font-bold text-gray-900 dark:text-gray-100">
                {formatCurrency(proceeds.total)}
              </span>
            </div>
          </div>
        </div>
      )}

      {/* Error Display */}
      {error && (
        <div className="mt-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md">
          <p className="text-sm text-red-600 dark:text-red-400">{error}</p>
        </div>
      )}

      {/* Success Display */}
      {success && (
        <div className="mt-4 p-3 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-md">
          <p className="text-sm text-green-600 dark:text-green-400">
            Treasury sold successfully!
          </p>
        </div>
      )}

      {/* Sell Button */}
      <div className="mt-6">
        <Button
          onClick={handleSell}
          disabled={!selectedHolding || !amount || amount <= 0 || loading || !currentUser}
          loading={loading}
          className="w-full"
        >
          {loading ? 'Processing...' : 'Sell Treasury'}
        </Button>
      </div>
    </Card>
  );
}
