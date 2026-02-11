import { useState, useEffect } from 'react';
import { useCurrentUser } from '../contexts/CurrentUserContext';
import { useTransactionRefresh } from '../contexts/TransactionRefreshContext';
import { Select, SelectItem, Button } from '@tremor/react';
import { QuestionMarkCircleIcon } from '@heroicons/react/24/outline';
import { TransactionModal } from './TransactionModal';
import { fetchUserHoldings } from '../services/api';
import type { TransactionType } from '../types/transaction';
import type { Holding } from '../types/holding';

interface HeaderProps {
  onStartTour?: () => void;
}

export function Header({ onStartTour }: HeaderProps) {
  const { users, currentUser, setCurrentUserById, refreshUser, isLoading } = useCurrentUser();
  const { refreshTransactions } = useTransactionRefresh();
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [modalType, setModalType] = useState<TransactionType>('fund');
  const [treasuriesBalance, setTreasuriesBalance] = useState<number>(0);
  const [loadingHoldings, setLoadingHoldings] = useState<boolean>(false);

  // Fetch holdings and calculate total treasuries balance
  useEffect(() => {
    const loadHoldingsBalance = async () => {
      if (!currentUser) {
        setTreasuriesBalance(0);
        return;
      }

      try {
        setLoadingHoldings(true);
        const holdings = await fetchUserHoldings(currentUser.id);
        // Sum up all remaining_amount (face value) from active holdings
        const total = holdings.reduce((sum: number, holding: Holding) => {
          const remainingAmount = parseFloat(holding.remaining_amount);
          return sum + (remainingAmount > 0 ? remainingAmount : 0);
        }, 0);
        setTreasuriesBalance(total);
      } catch (error) {
        console.error('Failed to fetch holdings for balance:', error);
        setTreasuriesBalance(0);
      } finally {
        setLoadingHoldings(false);
      }
    };

    loadHoldingsBalance();
  }, [currentUser]);

  if (isLoading) {
    return (
      <header className="bg-white dark:bg-gray-900 border-b border-gray-200 dark:border-gray-800 shadow-sm px-6 py-4">
        <div className="animate-pulse flex items-center space-x-4">
          <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded w-32"></div>
          <div className="h-8 bg-gray-200 dark:bg-gray-700 rounded w-48"></div>
          <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded w-40"></div>
        </div>
      </header>
    );
  }

  return (
    <header className="bg-white dark:bg-gray-900 border-b border-gray-200 dark:border-gray-800 shadow-sm px-4 sm:px-6 py-4">
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4 md:gap-6">
        {/* Logo/Title Section */}
        <div className="flex items-center gap-2">
          <h1 className="text-xl font-semibold text-gray-900 dark:text-gray-100">Dylan's Treasury Dashboard</h1>
        </div>

        {/* Right Section: User Dropdown, Balance, Actions */}
        <div className="flex flex-col sm:flex-row sm:items-center gap-3 sm:gap-4">
          {/* User Dropdown */}
          <div className="flex items-center gap-2" data-tour-id="user-selector">
            <span className="text-sm text-gray-600 dark:text-gray-400 whitespace-nowrap">User:</span>
            <Select
              value={currentUser?.id.toString() || ''}
              onValueChange={(value) => setCurrentUserById(parseInt(value, 10))}
              className="w-full sm:w-48"
            >
              {users.map((user) => (
                <SelectItem key={user.id} value={user.id.toString()}>
                  {user.name}
                </SelectItem>
              ))}
            </Select>
          </div>

          {/* Balance Displays */}
          <div className="flex flex-col sm:flex-row gap-2">
            {/* Cash Balance */}
            <div className="flex items-center gap-2 px-3 py-1.5 bg-gray-50 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700" data-tour-id="balance-display">
              <span className="text-sm text-gray-600 dark:text-gray-400 whitespace-nowrap">Cash Balance:</span>
              <div className="text-lg font-semibold text-gray-900 dark:text-gray-100">
                {currentUser
                  ? `$${currentUser.balance.toLocaleString('en-US', {
                      minimumFractionDigits: 2,
                      maximumFractionDigits: 2,
                    })}`
                  : '-'}
              </div>
            </div>

            {/* Treasuries Balance */}
            <div className="flex items-center gap-2 px-3 py-1.5 bg-blue-50 dark:bg-blue-900/20 rounded-lg border border-blue-200 dark:border-blue-800">
              <span className="text-sm text-blue-600 dark:text-blue-400 whitespace-nowrap">Treasuries Balance:</span>
              <div className="text-lg font-semibold text-blue-900 dark:text-blue-100">
                {loadingHoldings ? (
                  <div className="h-6 w-24 bg-blue-200 dark:bg-blue-800 rounded animate-pulse"></div>
                ) : (
                  `$${treasuriesBalance.toLocaleString('en-US', {
                    minimumFractionDigits: 2,
                    maximumFractionDigits: 2,
                  })}`
                )}
              </div>
            </div>
          </div>

          {/* Action Buttons */}
          <div className="flex gap-2" data-tour-id="fund-withdraw-buttons">
            {/* Tour Button */}
            {onStartTour && (
              <button
                onClick={onStartTour}
                className="flex items-center justify-center p-2 text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
                aria-label="View app walkthrough"
                title="View app walkthrough"
              >
                <QuestionMarkCircleIcon className="h-6 w-6" />
              </button>
            )}

            <Button
              onClick={() => {
                setModalType('fund');
                setIsModalOpen(true);
              }}
              variant="primary"
              className="flex-1 sm:flex-none"
            >
              Fund
            </Button>

            <Button
              onClick={() => {
                setModalType('withdraw');
                setIsModalOpen(true);
              }}
              variant="secondary"
              className="flex-1 sm:flex-none"
            >
              Withdraw
            </Button>
          </div>
        </div>
      </div>

      <TransactionModal
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        type={modalType}
        currentBalance={currentUser?.balance.toString() || '0'}
        userId={currentUser?.id || 0}
        onSuccess={async () => {
          await refreshUser();
          refreshTransactions();
          // Refresh treasuries balance after transaction
          if (currentUser) {
            try {
              const holdings = await fetchUserHoldings(currentUser.id);
              const total = holdings.reduce((sum: number, holding: Holding) => {
                const remainingAmount = parseFloat(holding.remaining_amount);
                return sum + (remainingAmount > 0 ? remainingAmount : 0);
              }, 0);
              setTreasuriesBalance(total);
            } catch (error) {
              console.error('Failed to refresh holdings balance:', error);
            }
          }
        }}
      />
    </header>
  );
}
