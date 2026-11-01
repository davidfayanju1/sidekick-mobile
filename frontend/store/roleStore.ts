import { create } from 'zustand';

import type { RoleIntent, User } from '@/lib/api/schemas';

export type UserRole = 'hero' | 'sidekick';
export type VerificationStatus = 'unverified' | 'pending' | 'verified' | 'rejected';

/**
 * The UI and the API use different words for the same thing. The API stores
 * `role_intent` as post/work/both; the product calls those Hero and Sidekick.
 * Translate at this boundary and nowhere else.
 */
export function roleFromIntent(intent: RoleIntent | null | undefined): UserRole | null {
  if (intent === 'post') return 'hero';
  if (intent === 'work') return 'sidekick';
  return null; // 'both' and null both mean "the user still has to pick"
}

export function intentFromRole(role: UserRole): RoleIntent {
  return role === 'hero' ? 'post' : 'work';
}

interface RoleState {
  role: UserRole | null;
  hasChosenRole: boolean;
  setRole: (role: UserRole) => void;

  /** Mirrors server state onto the local role/verification flags after /me loads. */
  syncFromUser: (user: User) => void;

  paymentMethodAdded: boolean;
  setPaymentMethodAdded: (added: boolean) => void;

  /** Mirrors `users.phone_verified`; drives the setup checklist. */
  phoneVerified: boolean;
  setPhoneVerified: (verified: boolean) => void;

  idVerificationStatus: VerificationStatus;
  setIdVerificationStatus: (status: VerificationStatus) => void;

  bankDetailsAdded: boolean;
  setBankDetailsAdded: (added: boolean) => void;
}

export const useRoleStore = create<RoleState>((set) => ({
  role: null,
  hasChosenRole: false,
  setRole: (role) => set({ role, hasChosenRole: true }),

  syncFromUser: (user) =>
    set((state) => {
      const serverRole = roleFromIntent(user.role_intent);
      return {
        // 'both' maps to no single side, so keep whichever the user last
        // switched to, and start them as a Hero if they have no preference yet
        // — a null role leaves the profile with no switcher and no way out.
        role: serverRole ?? state.role ?? (user.role_intent === 'both' ? 'hero' : null),
        hasChosenRole: state.hasChosenRole || user.role_intent != null,
        idVerificationStatus: user.verification_status ?? state.idVerificationStatus,
        phoneVerified: user.phone_verified === true,
      };
    }),

  paymentMethodAdded: false,
  setPaymentMethodAdded: (paymentMethodAdded) => set({ paymentMethodAdded }),

  phoneVerified: false,
  setPhoneVerified: (phoneVerified) => set({ phoneVerified }),

  idVerificationStatus: 'unverified',
  setIdVerificationStatus: (idVerificationStatus) => set({ idVerificationStatus }),

  bankDetailsAdded: false,
  setBankDetailsAdded: (bankDetailsAdded) => set({ bankDetailsAdded }),
}));

export function useOutstandingStepsCount() {
  return useRoleStore((state) => {
    // Every role owes a verified phone; the rest depends on which side they're on.
    const phone = state.phoneVerified ? 0 : 1;
    if (state.role === 'sidekick') {
      return (
        phone +
        (state.idVerificationStatus === 'verified' ? 0 : 1) +
        (state.bankDetailsAdded ? 0 : 1)
      );
    }
    if (state.role === 'hero') {
      return phone + (state.paymentMethodAdded ? 0 : 1);
    }
    return 0;
  });
}
