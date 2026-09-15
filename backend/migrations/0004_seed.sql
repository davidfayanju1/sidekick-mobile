-- Phase 0 — 0004 app_config + categories seed (Testing Gate #11).
-- Placeholders are intentional: values are config rows, never hardcoded constants.

INSERT INTO app_config (key, value) VALUES
  ('fee_percent',                    '8'),
  ('fee_payer',                      '"poster"'),
  ('min_budget',                     '500'),
  ('max_budget',                     '50000'),
  ('auto_release_hours',             '72'),
  ('auto_release_warning_hours',     '24'),
  ('no_offer_nudge_hours',           '24'),
  ('chat_freeze_days',               '7'),
  ('review_publish_days',            '14'),
  ('cancellation_free_window_minutes','60'),
  ('cancellation_compensation_percent','20'),
  ('locale',                         '"en-GB"'),
  ('currency',                       '"GBP"'),
  ('otp_ttl_seconds',                '300'),
  ('otp_max_attempts',               '5'),
  ('otp_resend_cooldown_seconds',    '60'),
  ('require_verification_to_post',   'false'),
  ('scheduler_driver',               '"go"')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

INSERT INTO categories (slug, name, icon, sort_order, active) VALUES
  ('cleaning',  'Cleaning',  'sparkles', 10, TRUE),
  ('assembly',  'Assembly',  'wrench',   20, TRUE),
  ('delivery',  'Delivery',  'package',  30, TRUE),
  ('moving',    'Moving',    'truck',    40, TRUE),
  ('shopping',  'Shopping',  'cart',     50, TRUE),
  ('gardening', 'Gardening', 'leaf',     60, TRUE),
  ('pets',      'Pets',      'paw',      70, TRUE),
  ('other',     'Other',     'dots',     99, TRUE)
ON CONFLICT (slug) DO NOTHING;
