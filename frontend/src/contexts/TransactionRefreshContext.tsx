import { createContext, useContext, useCallback, useRef, type ReactNode } from 'react';
import type { TransactionHistoryRef } from '../components/TransactionHistory';
import type { CurrentHoldingsRef } from '../components/CurrentHoldings';

interface TransactionRefreshContextType {
  refreshTransactions: () => void;
  transactionHistoryRef: React.RefObject<TransactionHistoryRef>;
  holdingsRef: React.RefObject<CurrentHoldingsRef>;
}

const TransactionRefreshContext = createContext<TransactionRefreshContextType | null>(null);

export function TransactionRefreshProvider({ children }: { children: ReactNode }) {
  const transactionHistoryRef = useRef<TransactionHistoryRef>(null);
  const holdingsRef = useRef<CurrentHoldingsRef>(null);

  const refreshTransactions = useCallback(() => {
    transactionHistoryRef.current?.refresh();
    holdingsRef.current?.refresh();
  }, []);

  return (
    <TransactionRefreshContext.Provider value={{ refreshTransactions, transactionHistoryRef, holdingsRef }}>
      {children}
    </TransactionRefreshContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useTransactionRefresh() {
  const context = useContext(TransactionRefreshContext);
  if (!context) {
    throw new Error('useTransactionRefresh must be used within TransactionRefreshProvider');
  }
  return context;
}
