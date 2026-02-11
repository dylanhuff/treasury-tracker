// Backend timestamps are UTC but lack 'Z' suffix - append it for correct parsing.
export function formatTransactionDate(timestamp: string): string {
  const utcTimestamp = timestamp.endsWith('Z') ? timestamp : timestamp + 'Z';
  const date = new Date(utcTimestamp);
  return date.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    hour12: true,
  });
}

export function formatCurrency(amount: string | number, decimals: number = 2): string {
  const num = typeof amount === 'string' ? parseFloat(amount) : amount;
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  }).format(num);
}

export function formatTransactionType(type: string): string {
  return type.charAt(0).toUpperCase() + type.slice(1);
}

// "100000.50" -> "100,000.50"
export function formatNumberInputWithCommas(value: string): string {
  const parts = value.split('.');
  parts[0] = parts[0].replace(/\B(?=(\d{3})+(?!\d))/g, ',');
  return parts.join('.');
}
