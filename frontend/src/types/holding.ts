import type { SecurityType } from './treasury';

export interface Holding {
  id: number;
  user_id: number;
  term: string;
  amount: string;
  yield_at_purchase: string;
  purchase_date: string;
  remaining_amount: string;
  face_value?: string;
  purchase_price?: string;
  security_type?: SecurityType | null;
}

export interface SellRequest {
  user_id: number;
  holding_id: number;
  amount: number;
}
