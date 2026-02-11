export const SecurityType = {
  Bill: 'bill',
  Note: 'note',
  Bond: 'bond',
} as const;

export type SecurityType = typeof SecurityType[keyof typeof SecurityType];

export type TreasuryTerm = '1M' | '3M' | '6M' | '1Y' | '2Y' | '5Y' | '10Y' | '30Y';

export const TERM_DAYS: Record<TreasuryTerm, number> = {
  '1M': 30,
  '3M': 90,
  '6M': 180,
  '1Y': 365,
  '2Y': 730,
  '5Y': 1825,
  '10Y': 3650,
  '30Y': 10950,
};

export function getSecurityType(term: string): SecurityType | null {
  switch (term) {
    case '1M':
    case '3M':
    case '6M':
    case '1Y':
      return SecurityType.Bill;
    case '2Y':
    case '5Y':
    case '10Y':
      return SecurityType.Note;
    case '30Y':
      return SecurityType.Bond;
    default:
      return null;
  }
}

export function getSecurityTypeName(type: SecurityType): string {
  switch (type) {
    case SecurityType.Bill:
      return 'Treasury Bill';
    case SecurityType.Note:
      return 'Treasury Note';
    case SecurityType.Bond:
      return 'Treasury Bond';
  }
}

export function getSecurityTypeBadgeColor(type: SecurityType): 'blue' | 'purple' | 'indigo' {
  switch (type) {
    case SecurityType.Bill:
      return 'blue';
    case SecurityType.Note:
      return 'purple';
    case SecurityType.Bond:
      return 'indigo';
  }
}

export function getSecurityTypeLabel(type: SecurityType): string {
  switch (type) {
    case SecurityType.Bill:
      return 'Bill';
    case SecurityType.Note:
      return 'Note';
    case SecurityType.Bond:
      return 'Bond';
  }
}

// Mirrors backend pricing: T-Bills use 360-day discount, Notes/Bonds use par.
export function calculatePurchasePrice(
  faceValue: number,
  yieldRate: number,
  term: TreasuryTerm,
  securityType: SecurityType | null
): number | null {
  if (faceValue <= 0 || yieldRate < 0 || yieldRate > 100) {
    return null;
  }

  const days = TERM_DAYS[term];
  if (!days || !securityType) {
    return null;
  }

  if (securityType === SecurityType.Bill) {
    // price = faceValue * (1 - (yieldRate/100 * days) / 360)
    const discountFactor = (yieldRate / 100.0 * days) / 360.0;
    const price = faceValue * (1.0 - discountFactor);
    return Math.round(price * 100) / 100;
  }

  if (securityType === SecurityType.Note || securityType === SecurityType.Bond) {
    return Math.round(faceValue * 100) / 100;
  }

  return null;
}

// Simple interest: principal + (principal * yield * years)
export function calculateMaturityValue(
  principal: number,
  yieldRate: number,
  daysHeld: number
): number {
  const simpleInterest = principal * (yieldRate / 100) * (daysHeld / 365);
  const maturityValue = principal + simpleInterest;
  return Math.round(maturityValue * 100) / 100;
}

export function getSecurityTypeTooltip(type: SecurityType): string {
  switch (type) {
    case SecurityType.Bill:
      return 'Treasury Bills are zero-coupon securities purchased at a discount. You pay less than face value and receive the full amount at maturity.';
    case SecurityType.Note:
    case SecurityType.Bond:
      return 'Treasury Notes and Bonds are purchased at face value and earn interest over time. This simplified model calculates total return at maturity.';
  }
}
