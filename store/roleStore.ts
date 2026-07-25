import { create } from 'zustand';

export type UserRole = 'hero' | 'sidekick';
export type VerificationStatus = 'unverified' | 'pending' | 'verified' | 'rejected';

interface RoleState {
  role: UserRole | null;
  hasChosenRole: boolean;
  setRole: (role: UserRole) => void;

  paymentMethodAdded: boolean;
  setPaymentMethodAdded: (added: boolean) => void;

  idVerificationStatus: VerificationStatus;
  setIdVerificationStatus: (status: VerificationStatus) => void;

  bankDetailsAdded: boolean;
  setBankDetailsAdded: (added: boolean) => void;
}

export const useRoleStore = create<RoleState>((set) => ({
  role: null,
  hasChosenRole: false,
  setRole: (role) => set({ role, hasChosenRole: true }),

  paymentMethodAdded: false,
  setPaymentMethodAdded: (paymentMethodAdded) => set({ paymentMethodAdded }),

  idVerificationStatus: 'unverified',
  setIdVerificationStatus: (idVerificationStatus) => set({ idVerificationStatus }),

  bankDetailsAdded: false,
  setBankDetailsAdded: (bankDetailsAdded) => set({ bankDetailsAdded }),
}));

export function useOutstandingStepsCount() {
  return useRoleStore((state) => {
    if (state.role === 'sidekick') {
      return (state.idVerificationStatus === 'verified' ? 0 : 1) + (state.bankDetailsAdded ? 0 : 1);
    }
    if (state.role === 'hero') {
      return state.paymentMethodAdded ? 0 : 1;
    }
    return 0;
  });
}
