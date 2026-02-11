import type { User } from '../types/user';
import type { Transaction, TransactionRequest, TransactionResponse, BuyRequest } from '../types/transaction';
import type { Holding, SellRequest } from '../types/holding';

export type { Holding, SellRequest } from '../types/holding';

export interface YieldPoint {
  term: string;
  rate: number;
}

export interface YieldData {
  date: string;
  yields: YieldPoint[];
}

export interface HistoricalDataPoint {
  date: string;
  "10Y": number;
  "5Y": number;
  "2Y": number;
}

export interface HistoricalYieldData {
  period: string;
  startDate: string;
  endDate: string;
  terms: string[];
  data: HistoricalDataPoint[];
}

// Relative URLs in production (nginx proxies to backend), full URL for local dev
const API_BASE_URL = import.meta.env.DEV
  ? (import.meta.env.VITE_API_URL || 'http://localhost:8080')
  : '';

export async function fetchYields(): Promise<YieldData> {
  try {
    const response = await fetch(`${API_BASE_URL}/api/yields`, {
      method: 'GET',
      headers: {
        'Accept': 'application/json',
      },
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const data: YieldData = await response.json();
    return data;
  } catch (error) {
    if (error instanceof Error) {
      throw new Error(`Failed to fetch yields: ${error.message}`);
    }
    throw new Error('Failed to fetch yields: Unknown error');
  }
}

export async function fetchHistoricalYields(
  period: string
): Promise<HistoricalYieldData> {
  try {
    const response = await fetch(
      `${API_BASE_URL}/api/yields/historical?period=${period}`,
      {
        method: 'GET',
        headers: {
          'Accept': 'application/json',
        },
      }
    );

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const data: HistoricalYieldData = await response.json();
    return data;
  } catch (error) {
    if (error instanceof Error) {
      throw new Error(`Failed to fetch historical yields: ${error.message}`);
    }
    throw new Error('Failed to fetch historical yields: Unknown error');
  }
}

export async function fetchUsers(): Promise<User[]> {
  try {
    const response = await fetch(`${API_BASE_URL}/api/v1/users`, {
      headers: {
        'Accept': 'application/json',
      },
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const data: User[] = await response.json();
    return data;
  } catch (error) {
    if (error instanceof Error) {
      throw new Error(`Failed to fetch users: ${error.message}`);
    }
    throw new Error('Failed to fetch users: Unknown error');
  }
}

export async function fundAccount(userId: number, amount: number): Promise<User> {
  const response = await fetch(`${API_BASE_URL}/api/v1/fund`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      user_id: userId,
      amount: amount,
    } as TransactionRequest),
  });

  const data: TransactionResponse = await response.json();

  if (!response.ok || !data.success) {
    throw new Error(data.error || 'Fund operation failed');
  }

  if (!data.user) {
    throw new Error('No user data returned');
  }

  return {
    id: data.user.id,
    name: data.user.name,
    balance: parseFloat(data.user.balance),
    created_at: data.user.created_at,
  };
}

export async function withdrawAccount(userId: number, amount: number): Promise<User> {
  const response = await fetch(`${API_BASE_URL}/api/v1/withdraw`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      user_id: userId,
      amount: amount,
    } as TransactionRequest),
  });

  const data: TransactionResponse = await response.json();

  if (!response.ok || !data.success) {
    throw new Error(data.error || 'Withdraw operation failed');
  }

  if (!data.user) {
    throw new Error('No user data returned');
  }

  return {
    id: data.user.id,
    name: data.user.name,
    balance: parseFloat(data.user.balance),
    created_at: data.user.created_at,
  };
}

export async function buyTreasury(
  userId: number,
  term: string,
  faceValue: number
): Promise<User> {
  const response = await fetch(`${API_BASE_URL}/api/v1/buy`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      user_id: userId,
      term: term,
      face_value: faceValue,
    } as BuyRequest),
  });

  const data: TransactionResponse = await response.json();

  if (!response.ok || !data.success) {
    throw new Error(data.error || 'Buy operation failed');
  }

  if (!data.user) {
    throw new Error('No user data returned');
  }

  return {
    id: data.user.id,
    name: data.user.name,
    balance: parseFloat(data.user.balance),
    created_at: data.user.created_at,
  };
}

export async function fetchUserTransactions(userId: number): Promise<Transaction[]> {
  try {
    const response = await fetch(`${API_BASE_URL}/api/v1/users/${userId}/transactions`, {
      method: 'GET',
      headers: {
        'Accept': 'application/json',
      },
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const data: Transaction[] = await response.json();
    return data;
  } catch (error) {
    if (error instanceof Error) {
      throw new Error(`Failed to fetch transactions: ${error.message}`);
    }
    throw new Error('Failed to fetch transactions: Unknown error');
  }
}

export async function fetchUserHoldings(userId: number): Promise<Holding[]> {
  try {
    const response = await fetch(`${API_BASE_URL}/api/v1/users/${userId}/holdings`, {
      method: 'GET',
      headers: {
        'Accept': 'application/json',
      },
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const data: Holding[] = await response.json();
    return data;
  } catch (error) {
    if (error instanceof Error) {
      throw new Error(`Failed to fetch holdings: ${error.message}`);
    }
    throw new Error('Failed to fetch holdings: Unknown error');
  }
}

export async function sellTreasury(
  userId: number,
  holdingId: number,
  amount: number
): Promise<User> {
  const response = await fetch(`${API_BASE_URL}/api/v1/sell`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      user_id: userId,
      holding_id: holdingId,
      amount: amount,
    } as SellRequest),
  });

  const data: TransactionResponse = await response.json();

  if (!response.ok || !data.success) {
    throw new Error(data.error || 'Sell operation failed');
  }

  if (!data.user) {
    throw new Error('No user data returned');
  }

  return {
    id: data.user.id,
    name: data.user.name,
    balance: parseFloat(data.user.balance),
    created_at: data.user.created_at,
  };
}
