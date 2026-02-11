import React, { useState } from 'react';
import { Dialog, DialogPanel } from '@headlessui/react';
import { Button } from '@tremor/react';
import type { TransactionType } from '../types/transaction';
import { fundAccount, withdrawAccount } from '../services/api';
import { formatNumberInputWithCommas } from '../utils/formatters';

interface TransactionModalProps {
  isOpen: boolean;
  onClose: () => void;
  type: TransactionType;
  currentBalance: string;
  userId: number;
  onSuccess: () => void;
}

export function TransactionModal({
  isOpen,
  onClose,
  type,
  currentBalance,
  userId,
  onSuccess,
}: TransactionModalProps) {
  const [amount, setAmount] = useState<number | undefined>(undefined);
  const [displayValue, setDisplayValue] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>('');

  const title = type === 'fund' ? 'Fund Account' : 'Withdraw Funds';
  const actionLabel = type === 'fund' ? 'Fund' : 'Withdraw';

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

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    // Validation
    if (!amount || amount <= 0) {
      setError('Amount must be greater than zero');
      return;
    }

    setLoading(true);

    try {
      // API call
      const apiFunction = type === 'fund' ? fundAccount : withdrawAccount;
      await apiFunction(userId, amount);

      // Success
      onSuccess();
      onClose();
      setAmount(undefined);
      setDisplayValue('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Transaction failed');
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    setAmount(undefined);
    setDisplayValue('');
    setError('');
    onClose();
  };

  return (
    <Dialog open={isOpen} onClose={handleClose} className="relative z-50">
      <div className="fixed inset-0 bg-black/30 dark:bg-black/60" aria-hidden="true" />

      <div className="fixed inset-0 flex items-center justify-center p-4">
        <DialogPanel className="mx-auto max-w-md rounded-lg bg-white dark:bg-gray-800 p-6 shadow-xl">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">{title}</h2>

          <div className="mb-4 text-sm text-gray-600 dark:text-gray-400">
            Current Balance: <span className="font-semibold text-gray-900 dark:text-gray-100">${parseFloat(currentBalance).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span>
          </div>

          <form onSubmit={handleSubmit}>
            <div className="mb-4">
              <label htmlFor="amount" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Amount
              </label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <span className="text-gray-500 dark:text-gray-400 text-base">$</span>
                </div>
                <input
                  id="amount"
                  type="text"
                  value={displayValue}
                  onChange={(e) => handleAmountChange(e.target.value)}
                  placeholder="Enter amount"
                  className="pl-7 pr-4 py-2 w-full text-base border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500"
                />
              </div>
            </div>

            {error && (
              <div className="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded text-sm text-red-700 dark:text-red-400">
                {error}
              </div>
            )}

            <div className="flex gap-3">
              <Button
                type="button"
                variant="secondary"
                onClick={handleClose}
                disabled={loading}
                className="flex-1"
              >
                Cancel
              </Button>
              <Button
                type="submit"
                variant="primary"
                loading={loading}
                disabled={loading}
                className="flex-1"
              >
                {actionLabel}
              </Button>
            </div>
          </form>
        </DialogPanel>
      </div>
    </Dialog>
  );
}
