export type TransactionType = 'fund' | 'withdraw' | 'buy' | 'sell';

export interface Transaction {
  id: number;
  user_id: number;
  timestamp: string;
  type: TransactionType;
  term: string | null;
  amount: string;
  yield_at_transaction: string | null;
  balance_after: string;
  holding_id: number | null;
}

export interface TransactionRequest {
  user_id: number;
  amount: number;
}

export interface BuyRequest {
  user_id: number;
  term: string;
  amount?: number;
  face_value: number;
}

export interface TransactionResponse {
  success: boolean;
  user?: {
    id: number;
    name: string;
    balance: string;
    created_at: string;
  };
  error?: string;
}
