import { useState, useEffect } from 'react';
import { Card, Title, Select, SelectItem, Button } from '@tremor/react';
import { useCurrentUser } from '../contexts/CurrentUserContext';
import { useTransactionRefresh } from '../contexts/TransactionRefreshContext';
import { buyTreasury, fetchYields, type YieldData } from '../services/api';
import { formatCurrency, formatNumberInputWithCommas } from '../utils/formatters';
import {
  SecurityType,
  type TreasuryTerm,
  TERM_DAYS,
  getSecurityType,
  getSecurityTypeName,
  getSecurityTypeTooltip,
  calculatePurchasePrice,
  calculateMaturityValue,
} from '../types/treasury';

const TREASURY_TERMS = [
  { value: '1M', label: '1 Month' },
  { value: '3M', label: '3 Months' },
  { value: '6M', label: '6 Months' },
  { value: '1Y', label: '1 Year' },
  { value: '2Y', label: '2 Years' },
  { value: '5Y', label: '5 Years' },
  { value: '10Y', label: '10 Years' },
  { value: '30Y', label: '30 Years' },
];

export function OrderForm() {
  const { currentUser, updateUser } = useCurrentUser();
  const { refreshTransactions } = useTransactionRefresh();

  const [selectedTerm, setSelectedTerm] = useState<string>('');
  const [amount, setAmount] = useState<number | undefined>(undefined);
  const [displayValue, setDisplayValue] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<boolean>(false);
  const [successMessage, setSuccessMessage] = useState<string>('');
  const [yieldData, setYieldData] = useState<YieldData | null>(null);
  const [currentYield, setCurrentYield] = useState<number | null>(null);
  const [purchasePrice, setPurchasePrice] = useState<number | null>(null);

  useEffect(() => {
    const loadYields = async () => {
      try {
        const data = await fetchYields();
        setYieldData(data);
      } catch {
        // Error handled silently - yield data not critical for form display
      }
    };
    loadYields();
  }, []);

  useEffect(() => {
    if (selectedTerm && yieldData) {
      const yieldPoint = yieldData.yields.find(y => y.term === selectedTerm);
      setCurrentYield(yieldPoint ? yieldPoint.rate : null);
    } else {
      setCurrentYield(null);
    }
  }, [selectedTerm, yieldData]);

  useEffect(() => {
    if (!amount || !selectedTerm || currentYield === null) {
      setPurchasePrice(null);
      return;
    }

    const securityType = getSecurityType(selectedTerm);
    const calculatedPrice = calculatePurchasePrice(
      amount,
      currentYield,
      selectedTerm as TreasuryTerm,
      securityType
    );
    setPurchasePrice(calculatedPrice);
  }, [amount, selectedTerm, currentYield]);

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
    if (!selectedTerm) {
      return 'Please select a treasury term';
    }

    if (!amount || amount <= 0) {
      return 'Please enter an amount greater than $0';
    }

    if (!currentUser) {
      return 'No user selected';
    }

    // For T-Bills, check purchase price against balance
    // For other terms, fall back to face value
    const costToCheck = purchasePrice !== null ? purchasePrice : amount;

    if (costToCheck > currentUser.balance) {
      if (purchasePrice !== null) {
        return `Insufficient balance. Purchase cost of ${formatCurrency(purchasePrice)} exceeds your available balance of ${formatCurrency(currentUser.balance, 0)}`;
      }
      return `Insufficient balance. You have ${formatCurrency(currentUser.balance, 0)} available`;
    }

    return null; // Valid
  };

  const handleBuy = async () => {
    if (!currentUser) return;

    // Clear previous state
    setError(null);
    setSuccess(false);

    // Validate form
    const validationError = validateForm();
    if (validationError) {
      setError(validationError);
      return;
    }

    // Get security type for better messaging
    const securityType = getSecurityType(selectedTerm);
    const securityTypeName = securityType ? getSecurityTypeName(securityType) : 'Treasury';

    try {
      setLoading(true);

      // Call API (amount is guaranteed to be defined by validation)
      const updatedUser = await buyTreasury(
        currentUser.id,
        selectedTerm,
        amount!
      );

      // Update user context (balance updates in header)
      updateUser(updatedUser);

      // Refresh transaction history
      await refreshTransactions();

      // Get term label for display
      const termLabel = TREASURY_TERMS.find(t => t.value === selectedTerm)?.label || selectedTerm;

      // Set success message with security type
      setSuccessMessage(`${securityTypeName} (${termLabel}) purchased successfully!`);

      // Clear form
      setSelectedTerm('');
      setAmount(undefined);
      setDisplayValue('');

      // Show success message
      setSuccess(true);

      // Auto-hide success message after 3 seconds
      setTimeout(() => {
        setSuccess(false);
        setSuccessMessage('');
      }, 3000);

    } catch (err) {
      // Enhanced error handling for both security types
      const errorMessage = err instanceof Error ? err.message : 'Failed to purchase treasury';

      // Add security type context to error message if not already present
      if (!errorMessage.toLowerCase().includes('bill') &&
          !errorMessage.toLowerCase().includes('note') &&
          !errorMessage.toLowerCase().includes('bond')) {
        setError(`${errorMessage} (${securityTypeName})`);
      } else {
        setError(errorMessage);
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card className="mt-6" data-tour-id="order-form">
      <Title>Buy Treasury</Title>

      {/* Current Balance Display */}
      <div className="mt-4">
        <p className="text-sm text-gray-600 dark:text-gray-400">
          Available Balance:{' '}
          <span className="font-semibold">
            {currentUser ? formatCurrency(currentUser.balance) : '-'}
          </span>
        </p>
      </div>

      {/* Term Selection */}
      <div className="mt-4">
        <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
          Treasury Term
        </label>
        <Select
          value={selectedTerm}
          onValueChange={setSelectedTerm}
          placeholder="Select term..."
          className="mt-1"
        >
          {TREASURY_TERMS.map((term) => (
            <SelectItem key={term.value} value={term.value}>
              {term.label}
            </SelectItem>
          ))}
        </Select>

        {/* Security Type Label - dynamically updates based on selected term */}
        {selectedTerm && (() => {
          const securityType = getSecurityType(selectedTerm);
          if (!securityType) return null;

          const tooltipText = getSecurityTypeTooltip(securityType);
          const typeName = getSecurityTypeName(securityType);

          return (
            <div className="mt-2">
              <p className="text-sm text-gray-600 dark:text-gray-400 flex items-center gap-1.5">
                Security Type:{' '}
                <span className="font-semibold">{typeName}</span>
                <span
                  className="inline-flex items-center justify-center w-4 h-4 text-xs rounded-full bg-blue-100 dark:bg-blue-900 text-blue-600 dark:text-blue-400 cursor-help"
                  title={tooltipText}
                  aria-label="Information about security type"
                >
                  ⓘ
                </span>
              </p>
            </div>
          );
        })()}

        {/* Current Yield Display */}
        {currentYield !== null && selectedTerm && (
          <div className="mt-2">
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Current Yield: <span className="font-semibold text-gray-900 dark:text-gray-100">{currentYield.toFixed(2)}%</span>
            </p>
          </div>
        )}
      </div>

      {/* Amount Input */}
      <div className="mt-4">
        <label className="text-sm font-medium text-gray-700 dark:text-gray-300 flex items-center gap-1.5">
          Face Value (Amount at Maturity)
          <span
            className="inline-flex items-center justify-center w-4 h-4 text-xs rounded-full bg-blue-100 dark:bg-blue-900 text-blue-600 dark:text-blue-400 cursor-help"
            title="Treasury Bills are purchased at a discount to face value. You pay less now and receive the full face value at maturity."
            aria-label="Information about face value"
          >
            ⓘ
          </span>
        </label>
        <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
          The amount you will receive when the bill matures
        </p>
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

      {/* Purchase Cost Display */}
      {purchasePrice !== null && (
        <div className="mt-2">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Purchase Cost:{' '}
            <span className="font-semibold text-gray-900 dark:text-gray-100">
              {formatCurrency(purchasePrice)}
            </span>
          </p>
        </div>
      )}

      {/* Information Display - Different for Bills vs Notes/Bonds */}
      {purchasePrice !== null && amount && currentYield !== null && selectedTerm && (() => {
        const securityType = getSecurityType(selectedTerm);
        if (!securityType) return null;

        // For Treasury Bills (1M-1Y): Show discount earned in green box
        if (securityType === SecurityType.Bill) {
          const discount = amount - purchasePrice;
          const returnPercent = (discount / amount * 100).toFixed(2);

          return (
            <div className="mt-3 p-3 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-md">
              <p className="text-sm font-semibold text-green-700 dark:text-green-400">
                Discount Earned: {formatCurrency(discount)}
                <span className="text-xs font-normal ml-1">
                  ({returnPercent}% return)
                </span>
              </p>
            </div>
          );
        }

        // For Treasury Notes/Bonds (2Y+): Show expected total return at maturity in blue box
        if (securityType === SecurityType.Note || securityType === SecurityType.Bond) {
          const days = TERM_DAYS[selectedTerm as TreasuryTerm];
          const years = days / 365;
          // Use centralized maturity value calculation
          const maturityValue = calculateMaturityValue(amount, currentYield, days);

          return (
            <div className="mt-3 p-3 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-md">
              <p className="text-sm font-semibold text-blue-700 dark:text-blue-400">
                Expected Total Return at Maturity: {formatCurrency(maturityValue)}
              </p>
              <p className="text-xs text-blue-600 dark:text-blue-500 mt-1">
                ({currentYield.toFixed(2)}% over {years.toFixed(1)} years)
              </p>
            </div>
          );
        }

        return null;
      })()}

      {/* Preview Balance After */}
      {amount && amount > 0 && currentUser && purchasePrice !== null && (
        <div className="mt-2">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            Balance after purchase:{' '}
            {formatCurrency(currentUser.balance - purchasePrice)}
          </p>
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
            {successMessage || 'Treasury purchased successfully!'}
          </p>
        </div>
      )}

      {/* Buy Button */}
      <div className="mt-6">
        <Button
          onClick={handleBuy}
          disabled={!selectedTerm || !amount || amount <= 0 || loading || !currentUser}
          loading={loading}
          className="w-full"
        >
          {loading ? 'Processing...' : 'Buy Treasury'}
        </Button>
      </div>
    </Card>
  );
}
