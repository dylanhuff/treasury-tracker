import { useState, useEffect, forwardRef, useImperativeHandle, useRef } from 'react';
import { Card, Title, Table, TableHead, TableHeaderCell, TableBody, TableRow, TableCell, Badge } from '@tremor/react';
import { useCurrentUser } from '../contexts/CurrentUserContext';
import { fetchUserHoldings } from '../services/api';
import type { Holding } from '../types/holding';
import { formatTransactionDate, formatCurrency } from '../utils/formatters';
import {
  SecurityType,
  getSecurityType as getSecurityTypeFromTerm,
  getSecurityTypeBadgeColor,
  getSecurityTypeLabel,
} from '../types/treasury';

// Define the ref handle type for external access
export interface CurrentHoldingsRef {
  refresh: () => Promise<void>;
}

// Helper functions for handling legacy holdings (null face_value/purchase_price)
const getFaceValue = (holding: Holding): string => {
  // For new T-Bill holdings, use face_value. For legacy holdings, fall back to amount.
  return holding.face_value ?? holding.amount;
};

const getPurchasePrice = (holding: Holding): string => {
  // For new T-Bill holdings, use purchase_price. For legacy holdings, fall back to amount.
  return holding.purchase_price ?? holding.amount;
};

const getDiscount = (holding: Holding): number => {
  // Calculate discount earned (face_value - purchase_price)
  const faceValue = parseFloat(getFaceValue(holding));
  const purchasePrice = parseFloat(getPurchasePrice(holding));
  return faceValue - purchasePrice;
};

const isNewTBill = (holding: Holding): boolean => {
  // Returns true if this is a new T-Bill holding with discount pricing
  return holding.face_value !== null && holding.face_value !== undefined;
};

// Helper function to infer security type from term (for legacy holdings with null security_type)
const getSecurityType = (holding: Holding): SecurityType => {
  // If security_type exists, use it
  if (holding.security_type) {
    return holding.security_type as SecurityType;
  }

  // Otherwise infer from term (for legacy holdings)
  const inferredType = getSecurityTypeFromTerm(holding.term);
  // Default to Bill if term is invalid (shouldn't happen in practice)
  return inferredType ?? SecurityType.Bill;
};

export const CurrentHoldings = forwardRef<CurrentHoldingsRef>((_, ref) => {
  const { currentUser } = useCurrentUser();
  const [holdings, setHoldings] = useState<Holding[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [showTooltip, setShowTooltip] = useState<boolean>(false);
  const [showReturnTooltip, setShowReturnTooltip] = useState<boolean>(false);
  const [showFade, setShowFade] = useState<boolean>(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  // Refresh function that can be called externally
  const refreshHoldings = async () => {
    if (!currentUser) return;

    try {
      setError(null);
      const data = await fetchUserHoldings(currentUser.id);
      // Filter to show only holdings with remaining_amount > 0
      const activeHoldings = data.filter(h => parseFloat(h.remaining_amount) > 0);
      setHoldings(activeHoldings);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch holdings');
    }
  };

  // Expose refresh method via ref
  useImperativeHandle(ref, () => ({
    refresh: refreshHoldings,
  }));

  useEffect(() => {
    if (!currentUser) {
      setHoldings([]);
      setLoading(false);
      return;
    }

    const loadHoldings = async () => {
      try {
        setLoading(true);
        setError(null);
        const data = await fetchUserHoldings(currentUser.id);
        // Filter to show only holdings with remaining_amount > 0
        const activeHoldings = data.filter(h => parseFloat(h.remaining_amount) > 0);
        setHoldings(activeHoldings);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to fetch holdings');
      } finally {
        setLoading(false);
      }
    };

    loadHoldings();
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
  }, [holdings]);

  // Loading state with skeleton
  if (loading) {
    return (
      <Card className="mt-6">
        <Title>Current Holdings</Title>
        <div className="mt-4 space-y-3 animate-pulse">
          {[...Array(5)].map((_, i) => (
            <div key={i} className="flex space-x-4">
              <div className="h-10 bg-gray-200 dark:bg-gray-700 rounded w-32"></div>
              <div className="h-10 bg-gray-200 dark:bg-gray-700 rounded w-20"></div>
              <div className="h-10 bg-gray-200 dark:bg-gray-700 rounded w-20"></div>
              <div className="h-10 bg-gray-200 dark:bg-gray-700 rounded w-32"></div>
              <div className="h-10 bg-gray-200 dark:bg-gray-700 rounded w-28"></div>
              <div className="h-10 bg-gray-200 dark:bg-gray-700 rounded w-24"></div>
              <div className="h-10 bg-gray-200 dark:bg-gray-700 rounded w-20"></div>
              <div className="h-10 bg-gray-200 dark:bg-gray-700 rounded w-28"></div>
            </div>
          ))}
        </div>
      </Card>
    );
  }

  // Error state
  if (error) {
    return (
      <Card className="mt-6">
        <Title>Current Holdings</Title>
        <div className="flex flex-col items-center justify-center h-48 space-y-4">
          <p className="text-red-600 dark:text-red-400 font-medium">{error}</p>
          <button
            onClick={() => window.location.reload()}
            className="px-4 py-2 bg-blue-500 dark:bg-blue-600 text-white rounded-md hover:bg-blue-600 dark:hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400 focus:ring-offset-2 dark:focus:ring-offset-gray-900 transition-all duration-200 shadow-sm hover:shadow-md"
            aria-label="Retry loading holdings"
          >
            Retry
          </button>
        </div>
      </Card>
    );
  }

  // Empty state
  if (holdings.length === 0) {
    return (
      <Card className="mt-6">
        <Title>Current Holdings</Title>
        <div className="flex items-center justify-center h-48">
          <p className="text-gray-500 dark:text-gray-400 text-center">
            No active holdings. Purchase treasuries to get started.
          </p>
        </div>
      </Card>
    );
  }

  // Success state with table
  return (
    <Card className="mt-6" data-tour-id="current-holdings">
      <div className="flex items-center justify-between mb-4">
        <Title>Current Holdings</Title>
        <div className="relative">
          <button
            onMouseEnter={() => setShowTooltip(true)}
            onMouseLeave={() => setShowTooltip(false)}
            className="text-blue-500 dark:text-blue-400 hover:text-blue-600 dark:hover:text-blue-300 transition-colors"
            aria-label="Treasury Bill discount pricing information"
          >
            <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </button>
          {showTooltip && (
            <div className="absolute right-0 top-8 z-10 w-80 p-3 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md shadow-lg text-sm">
              <p className="text-gray-700 dark:text-gray-300">
                <strong>T-Bill Discount Pricing:</strong> Treasury Bills are purchased at a discount to face value.
                You pay less now (Purchase Price) and receive the full Face Value at maturity. The difference is your earned discount.
              </p>
            </div>
          )}
        </div>
      </div>
      <div className="relative mt-4">
        <div
          ref={scrollRef}
          className="scrollable-holdings max-h-96 overflow-auto border border-gray-200 dark:border-gray-700 rounded-lg shadow-inner"
        >
          <Table>
            <TableHead>
              <TableRow>
                <TableHeaderCell className="text-gray-900 dark:text-gray-100">Purchase Date</TableHeaderCell>
                <TableHeaderCell className="text-gray-900 dark:text-gray-100">Term</TableHeaderCell>
                <TableHeaderCell className="text-gray-900 dark:text-gray-100">Type</TableHeaderCell>
                <TableHeaderCell className="text-gray-900 dark:text-gray-100">
                  Face Value (Maturity Amount)
                </TableHeaderCell>
                <TableHeaderCell className="text-gray-900 dark:text-gray-100">Purchase Price</TableHeaderCell>
                <TableHeaderCell className="text-gray-900 dark:text-gray-100">
                  <div className="flex items-center gap-1">
                    Return
                    <button
                      onMouseEnter={() => setShowReturnTooltip(true)}
                      onMouseLeave={() => setShowReturnTooltip(false)}
                      className="relative text-blue-500 dark:text-blue-400 hover:text-blue-600 dark:hover:text-blue-300 transition-colors"
                      aria-label="Return information"
                    >
                      <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                      </svg>
                      {showReturnTooltip && (
                        <div className="absolute left-1/2 -translate-x-1/2 top-6 z-10 w-64 p-2 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md shadow-lg text-xs text-left font-normal whitespace-normal">
                          <p className="text-gray-700 dark:text-gray-300">
                            Bills show discount earned (face value - purchase price). Notes/Bonds earn interest over time, shown at maturity.
                          </p>
                        </div>
                      )}
                    </button>
                  </div>
                </TableHeaderCell>
                <TableHeaderCell className="text-gray-900 dark:text-gray-100">Yield</TableHeaderCell>
                <TableHeaderCell className="text-gray-900 dark:text-gray-100">Remaining</TableHeaderCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {holdings.map((holding) => {
                const discount = getDiscount(holding);
                const isNew = isNewTBill(holding);
                const securityType = getSecurityType(holding);
                const badgeColor = getSecurityTypeBadgeColor(securityType);
                const typeLabel = getSecurityTypeLabel(securityType);

                return (
                  <TableRow
                    key={holding.id}
                    className="hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors duration-150"
                  >
                    <TableCell className="text-gray-700 dark:text-gray-300">
                      {formatTransactionDate(holding.purchase_date)}
                    </TableCell>
                    <TableCell>
                      <Badge
                        color="blue"
                        className="transition-all duration-200"
                      >
                        {holding.term}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge
                        color={badgeColor}
                        className="transition-all duration-200"
                      >
                        {typeLabel}
                      </Badge>
                    </TableCell>
                    <TableCell className="font-medium text-gray-900 dark:text-gray-100">
                      {formatCurrency(getFaceValue(holding))}
                    </TableCell>
                    <TableCell className="text-gray-700 dark:text-gray-300">
                      {formatCurrency(getPurchasePrice(holding))}
                    </TableCell>
                    <TableCell className={`font-medium ${
                      securityType === SecurityType.Bill
                        ? (isNew && discount > 0 ? 'text-green-600 dark:text-green-400' : 'text-gray-600 dark:text-gray-400')
                        : 'text-gray-500 dark:text-gray-500'
                    }`}>
                      {securityType === SecurityType.Bill
                        ? (isNew && discount > 0 ? `+${formatCurrency(discount.toString())}` : '-')
                        : 'At Maturity'}
                    </TableCell>
                    <TableCell className="text-gray-600 dark:text-gray-400">
                      {parseFloat(holding.yield_at_purchase).toFixed(2)}%
                    </TableCell>
                    <TableCell className="font-semibold text-gray-900 dark:text-gray-100">
                      {formatCurrency(holding.remaining_amount)}
                    </TableCell>
                  </TableRow>
                );
              })}
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
CurrentHoldings.displayName = 'CurrentHoldings';
