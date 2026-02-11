import { useState, useEffect, forwardRef, useImperativeHandle, useRef } from 'react';
import { Card, Title, Table, TableHead, TableHeaderCell, TableBody, TableRow, TableCell, Badge } from '@tremor/react';
import { useCurrentUser } from '../contexts/CurrentUserContext';
import { fetchUserTransactions } from '../services/api';
import type { Transaction } from '../types/transaction';
import { formatTransactionDate, formatCurrency, formatTransactionType } from '../utils/formatters';

// Define the ref handle type for external access
export interface TransactionHistoryRef {
  refresh: () => Promise<void>;
}

export const TransactionHistory = forwardRef<TransactionHistoryRef>((_, ref) => {
  const { currentUser } = useCurrentUser();
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [showFade, setShowFade] = useState<boolean>(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  // Refresh function that can be called externally
  const refreshTransactions = async () => {
    if (!currentUser) return;

    try {
      setError(null);
      const data = await fetchUserTransactions(currentUser.id);
      setTransactions(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch transactions');
    }
  };

  // Expose refresh method via ref
  useImperativeHandle(ref, () => ({
    refresh: refreshTransactions,
  }));

  useEffect(() => {
    if (!currentUser) {
      setTransactions([]);
      setLoading(false);
      return;
    }

    const loadTransactions = async () => {
      try {
        setLoading(true);
        setError(null);
        const data = await fetchUserTransactions(currentUser.id);
        setTransactions(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to fetch transactions');
      } finally {
        setLoading(false);
      }
    };

    loadTransactions();
  }, [currentUser]); // Re-fetch when user changes

  // Check if content is scrollable and update fade visibility
  useEffect(() => {
    const checkScroll = () => {
      const el = scrollRef.current;
      if (!el) return;

      const isScrollable = el.scrollHeight > el.clientHeight;
      const isAtBottom = Math.abs(el.scrollHeight - el.clientHeight - el.scrollTop) < 1;

      setShowFade(isScrollable && !isAtBottom);
    };

    checkScroll();
    const el = scrollRef.current;
    el?.addEventListener('scroll', checkScroll);
    window.addEventListener('resize', checkScroll);

    return () => {
      el?.removeEventListener('scroll', checkScroll);
      window.removeEventListener('resize', checkScroll);
    };
  }, [transactions]);

  const getTypeBadgeColor = (type: string): 'emerald' | 'rose' | 'blue' | 'violet' => {
    switch (type) {
      case 'fund':
        return 'emerald';
      case 'withdraw':
        return 'rose';
      case 'buy':
        return 'blue';
      case 'sell':
        return 'violet';
      default:
        return 'blue';
    }
  };

  // Loading state with skeleton
  if (loading) {
    return (
      <Card className="mt-8">
        <Title>Transaction History</Title>
        <div className="mt-4 space-y-3 animate-pulse">
          {[...Array(5)].map((_, i) => (
            <div key={i} className="flex space-x-4">
              <div className="h-10 bg-gray-200 dark:bg-gray-700 rounded w-32"></div>
              <div className="h-10 bg-gray-200 dark:bg-gray-700 rounded w-24"></div>
              <div className="h-10 bg-gray-200 dark:bg-gray-700 rounded w-20"></div>
              <div className="h-10 bg-gray-200 dark:bg-gray-700 rounded flex-1"></div>
              <div className="h-10 bg-gray-200 dark:bg-gray-700 rounded w-24"></div>
              <div className="h-10 bg-gray-200 dark:bg-gray-700 rounded w-32"></div>
            </div>
          ))}
        </div>
      </Card>
    );
  }

  // Error state
  if (error) {
    return (
      <Card className="mt-8">
        <Title>Transaction History</Title>
        <div className="flex flex-col items-center justify-center h-48 space-y-4">
          <p className="text-red-600 dark:text-red-400 font-medium">{error}</p>
          <button
            onClick={() => window.location.reload()}
            className="px-4 py-2 bg-blue-500 dark:bg-blue-600 text-white rounded-md hover:bg-blue-600 dark:hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400 focus:ring-offset-2 dark:focus:ring-offset-gray-900 transition-all duration-200 shadow-sm hover:shadow-md"
            aria-label="Retry loading transactions"
          >
            Retry
          </button>
        </div>
      </Card>
    );
  }

  // Empty state
  if (transactions.length === 0) {
    return (
      <Card className="mt-8">
        <Title>Transaction History</Title>
        <div className="flex items-center justify-center h-48">
          <p className="text-gray-500 dark:text-gray-400 text-center">
            No transactions yet. Fund or withdraw to get started.
          </p>
        </div>
      </Card>
    );
  }

  // Success state with table
  return (
    <Card className="mt-8" data-tour-id="transaction-history">
      <Title>Transaction History</Title>
      <div className="relative mt-4">
        <div
          ref={scrollRef}
          className="scrollable-holdings max-h-96 overflow-auto border border-gray-200 dark:border-gray-700 rounded-lg shadow-inner"
        >
          <Table>
            <TableHead>
              <TableRow>
                <TableHeaderCell className="text-gray-900 dark:text-gray-100">Date</TableHeaderCell>
                <TableHeaderCell className="text-gray-900 dark:text-gray-100">Type</TableHeaderCell>
                <TableHeaderCell className="text-gray-900 dark:text-gray-100">Term</TableHeaderCell>
                <TableHeaderCell className="text-gray-900 dark:text-gray-100">Amount</TableHeaderCell>
                <TableHeaderCell className="text-gray-900 dark:text-gray-100">Yield</TableHeaderCell>
                <TableHeaderCell className="text-gray-900 dark:text-gray-100">Balance After</TableHeaderCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {transactions.map((transaction) => (
                <TableRow
                  key={transaction.id}
                  className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors duration-150"
                >
                  <TableCell className="text-gray-700 dark:text-gray-300">
                    {formatTransactionDate(transaction.timestamp)}
                  </TableCell>
                  <TableCell>
                    <Badge
                      color={getTypeBadgeColor(transaction.type)}
                      className="transition-all duration-200"
                    >
                      {formatTransactionType(transaction.type)}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-gray-600 dark:text-gray-400">
                    {transaction.term || '—'}
                  </TableCell>
                  <TableCell className="font-medium text-gray-900 dark:text-gray-100">
                    {formatCurrency(transaction.amount)}
                  </TableCell>
                  <TableCell className="text-gray-600 dark:text-gray-400">
                    {transaction.yield_at_transaction
                      ? `${parseFloat(transaction.yield_at_transaction).toFixed(2)}%`
                      : '—'}
                  </TableCell>
                  <TableCell className="font-semibold text-gray-900 dark:text-gray-100">
                    {formatCurrency(transaction.balance_after)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
        {/* Gradient fade to indicate more content below */}
        {showFade && (
          <div className="absolute bottom-0 left-0 right-0 h-12 bg-gradient-to-t from-white dark:from-gray-900 to-transparent pointer-events-none rounded-b-lg"></div>
        )}
      </div>
    </Card>
  );
});

// Add display name for debugging
TransactionHistory.displayName = 'TransactionHistory';
