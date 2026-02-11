interface TourStep {
  target: string;
  content: string;
  placement?: 'top' | 'bottom' | 'left' | 'right' | 'center';
  disableBeacon?: boolean;
}

export const TOUR_STEPS: TourStep[] = [
  {
    target: 'body',
    content: 'Welcome to the Treasury Management Dashboard! This guided tour will walk you through all features including user management, treasury investments, portfolio tracking, and transaction history.',
    placement: 'center',
    disableBeacon: true,
  },
  {
    target: '[data-tour-id="user-selector"]',
    content: 'Switch between different user accounts to view their individual portfolios and balances. Each user has their own treasury holdings and transaction history.',
    placement: 'bottom',
    disableBeacon: true,
  },
  {
    target: '[data-tour-id="balance-display"]',
    content: 'Your account balances are displayed here. Cash Balance shows your available funds for purchasing treasuries, while Treasuries Balance shows the total face value of your treasury portfolio. Both update in real-time as you transact.',
    placement: 'bottom',
    disableBeacon: true,
  },
  {
    target: '[data-tour-id="fund-withdraw-buttons"]',
    content: 'Add funds to your account or withdraw money at any time. These actions are tracked in your transaction history for complete transparency.',
    placement: 'bottom',
    disableBeacon: true,
  },
  {
    target: '[data-tour-id="yield-curve-chart"]',
    content: 'Live treasury yield data is fetched from the U.S. Treasury API and visualized here. The curve updates daily and shows current rates across all maturity terms (1 month to 30 years).',
    placement: 'bottom',
    disableBeacon: true,
  },
  {
    target: '[data-tour-id="order-form"]',
    content: 'Purchase T-Bills, T-Notes, or T-Bonds by selecting a term and entering the face value amount. Purchase amounts are validated against your available balance to ensure you have sufficient funds.',
    placement: 'left',
    disableBeacon: true,
  },
  {
    target: '[data-tour-id="current-holdings"]',
    content: 'View all your active treasury holdings with details like purchase price, face value, yield rates, and maturity dates. Holdings are updated in real-time as you buy or sell.',
    placement: 'top',
    disableBeacon: true,
  },
  {
    target: '[data-tour-id="sell-form"]',
    content: 'Sell your treasury holdings before maturity. The system calculates your proceeds including principal and prorated yield based on how long you\'ve held the security.',
    placement: 'left',
    disableBeacon: true,
  },
  {
    target: '[data-tour-id="transaction-history"]',
    content: 'All transactions are permanently recorded here, including buys, sells, deposits, and withdrawals. This provides a complete audit trail of all account activity.',
    placement: 'top',
    disableBeacon: true,
  },
];
