import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { idempotencyKey, qk } from '@/lib/api/keys';
import type {
  PaymentMethod,
  PayoutMethod,
  Receipt,
  Transaction,
  Wallet,
  Withdrawal,
} from '@/lib/api/schemas';
import { api } from '@/lib/axios';

/**
 * Stand-in for a real provider token.
 *
 * The server runs `payments.Mock` while provider selection is open
 * (BACKEND.md §26 Q3). Minting it here rather than in the screen keeps the
 * impurity inside the mutation, and leaves exactly one place to delete once a
 * provider SDK is wired up.
 */
function sandboxToken(): string {
  return `sandbox_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
}

export function useWallet() {
  return useQuery({
    queryKey: qk.finance.wallet,
    queryFn: async () => {
      const { data } = await api.get<Wallet>('/me/wallet');
      return data;
    },
  });
}

export function useTransactions() {
  return useQuery({
    queryKey: qk.finance.transactions,
    queryFn: async () => {
      const { data } = await api.get<{ transactions: Transaction[] }>('/me/transactions');
      return data.transactions ?? [];
    },
  });
}

export function useReceipt(transactionId: string | undefined) {
  return useQuery({
    queryKey: qk.finance.receipt(transactionId ?? ''),
    enabled: !!transactionId,
    queryFn: async () => {
      const { data } = await api.get<Receipt>(`/transactions/${transactionId}/receipt`);
      return data;
    },
  });
}

export function usePaymentMethods() {
  return useQuery({
    queryKey: qk.finance.paymentMethods,
    queryFn: async () => {
      const { data } = await api.get<{ payment_methods: PaymentMethod[] }>('/me/payment-methods');
      return data.payment_methods ?? [];
    },
  });
}

/**
 * Registers a card that the payment provider has already tokenised.
 *
 * `provider_ref` is a token, never a card number — raw PANs must not reach
 * this app's servers, so the card form hands its digits to the provider SDK
 * and only the token comes back here.
 */
export function useAddPaymentMethod() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (body: {
      /** The provider SDK's token. Omit it and a sandbox stand-in is minted. */
      provider_ref?: string;
      last4: string;
      brand?: string;
      exp_month?: number;
      exp_year?: number;
    }) => {
      const { data } = await api.post<{ payment_method: PaymentMethod }>('/me/payment-methods', {
        ...body,
        provider_ref: body.provider_ref ?? sandboxToken(),
      });
      return data.payment_method;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: qk.finance.paymentMethods }),
  });
}

export function useRemovePaymentMethod() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      await api.delete(`/me/payment-methods/${id}`);
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: qk.finance.paymentMethods }),
  });
}

export function usePayoutMethods() {
  return useQuery({
    queryKey: qk.finance.payoutMethods,
    queryFn: async () => {
      const { data } = await api.get<{ payout_methods: PayoutMethod[] }>('/me/payout-methods');
      return data.payout_methods ?? [];
    },
  });
}

export function useAddPayoutMethod() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (body: {
      /** The provider SDK's token. Omit it and a sandbox stand-in is minted. */
      bank_ref_token?: string;
      bank_ref?: string;
      last4?: string;
      bank_name?: string;
    }) => {
      const { data } = await api.post<{ payout_method: PayoutMethod }>('/me/payout-methods', {
        ...body,
        bank_ref_token: body.bank_ref_token ?? sandboxToken(),
      });
      return data.payout_method;
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: qk.finance.payoutMethods }),
  });
}

/** Withdrawals are idempotent server-side — a retried request can't pay out twice. */
export function useWithdraw() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ amount, payoutMethodId }: { amount: number; payoutMethodId: string }) => {
      const { data } = await api.post<{ withdrawal: Withdrawal }>('/me/withdrawals', {
        amount,
        payout_method_id: payoutMethodId,
        idempotency_key: idempotencyKey(),
      });
      return data.withdrawal;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: qk.finance.wallet });
      queryClient.invalidateQueries({ queryKey: qk.finance.transactions });
    },
  });
}
