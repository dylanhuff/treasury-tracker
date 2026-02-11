import { useState, useEffect } from 'react'
import YieldCurve from './components/YieldCurve'
import { CurrentUserProvider } from './contexts/CurrentUserContext'
import { TransactionRefreshProvider, useTransactionRefresh } from './contexts/TransactionRefreshContext'
import { Header } from './components/Header'
import { TransactionHistory } from './components/TransactionHistory'
import { CurrentHoldings } from './components/CurrentHoldings'
import { OrderForm } from './components/OrderForm'
import { SellForm } from './components/SellForm'
import { Tour } from './components/Tour'
import { getTourCompleted, setTourCompleted } from './tour/storage'

function AppContent() {
  const { transactionHistoryRef, holdingsRef } = useTransactionRefresh();
  const [isAppReady, setIsAppReady] = useState(false);
  const [isTourRunning, setIsTourRunning] = useState(false);

  useEffect(() => {
    const timer = setTimeout(() => setIsAppReady(true), 500);
    return () => clearTimeout(timer);
  }, []);

  useEffect(() => {
    if (isAppReady && !getTourCompleted()) {
      setIsTourRunning(true);
    }
  }, [isAppReady]);

  const handleManualTourStart = () => {
    if (isAppReady) {
      setIsTourRunning(true);
    }
  };

  const handleTourComplete = () => {
    setIsTourRunning(false);
    setTourCompleted(true);
  };

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-950">
      <Header onStartTour={handleManualTourStart} />
      <div className="p-4 md:p-8">
        <div className="max-w-6xl mx-auto">
          <YieldCurve />
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mt-6">
            <OrderForm />
            <SellForm />
          </div>
          <CurrentHoldings ref={holdingsRef} />
          <TransactionHistory ref={transactionHistoryRef} />
        </div>
      </div>
      <Tour run={isTourRunning} onComplete={handleTourComplete} />
    </div>
  );
}

function App() {
  return (
    <CurrentUserProvider>
      <TransactionRefreshProvider>
        <AppContent />
      </TransactionRefreshProvider>
    </CurrentUserProvider>
  )
}

export default App
