import * as React from 'react';
import type { z } from 'zod';

type Errors<T> = Partial<Record<keyof T & string, string>>;

/**
 * Minimal zod-backed form state.
 *
 * Deliberately not react-hook-form: every input in this app is already a
 * controlled `<Form value onChangeText />`, so all that's actually missing is
 * validation, touched-tracking and a submit gate. That's this file.
 *
 * Errors stay hidden until a field has been blurred or submit has been
 * attempted — validating while someone is still typing their email reads as
 * nagging, not helpfulness. Once a field *has* errored, it re-validates on
 * every keystroke so the message clears the moment it's fixed.
 */
export function useZodForm<Schema extends z.ZodType<any, any>>(
  schema: Schema,
  initialValues: z.input<Schema> & Record<string, unknown>
) {
  type Input = z.input<Schema> & Record<string, unknown>;
  type Output = z.output<Schema>;
  type Key = keyof Input & string;

  const [values, setValues] = React.useState<Input>(initialValues);
  const [errors, setErrors] = React.useState<Errors<Input>>({});
  const [touched, setTouched] = React.useState<Partial<Record<Key, boolean>>>({});
  const [submitted, setSubmitted] = React.useState(false);

  const collect = React.useCallback(
    (next: Input): Errors<Input> => {
      const result = schema.safeParse(next);
      if (result.success) return {};
      const collected: Errors<Input> = {};
      for (const issue of result.error.issues) {
        const key = issue.path[0] as Key | undefined;
        // First issue per field wins — stacking three messages under one input
        // is noise; fix the first and the next one surfaces.
        if (key && !collected[key]) collected[key] = issue.message;
      }
      return collected;
    },
    [schema]
  );

  const setValue = React.useCallback(
    (key: Key, value: unknown) => {
      setValues((current) => {
        const next = { ...current, [key]: value };
        setErrors((previous) => {
          if (!previous[key] && !submitted) return previous;
          const fresh = collect(next);
          return { ...previous, [key]: fresh[key] };
        });
        return next;
      });
    },
    [collect, submitted]
  );

  const blur = React.useCallback(
    (key: Key) => {
      setTouched((current) => ({ ...current, [key]: true }));
      setErrors((previous) => ({ ...previous, [key]: collect(values)[key] }));
    },
    [collect, values]
  );

  /** Everything an input needs, in one spread: `<Form {...form.field('email')} />`. */
  const field = React.useCallback(
    (key: Key) => ({
      value: (values[key] ?? '') as string,
      onChangeText: (text: string) => setValue(key, text),
      onBlur: () => blur(key),
      error: touched[key] || submitted ? errors[key] : undefined,
    }),
    [values, errors, touched, submitted, setValue, blur]
  );

  /** Runs `onValid` with the *parsed* output — trimmed, coerced, pence not pounds. */
  const handleSubmit = React.useCallback(
    (onValid: (parsed: Output) => void | Promise<void>) => async () => {
      setSubmitted(true);
      const result = schema.safeParse(values);
      if (!result.success) {
        setErrors(collect(values));
        return;
      }
      setErrors({});
      await onValid(result.data as Output);
    },
    [schema, values, collect]
  );

  const reset = React.useCallback(() => {
    setValues(initialValues);
    setErrors({});
    setTouched({});
    setSubmitted(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return {
    values,
    errors,
    field,
    setValue,
    setValues,
    handleSubmit,
    reset,
    /** True once the whole form would parse — use it to enable the submit button. */
    isValid: React.useMemo(() => schema.safeParse(values).success, [schema, values]),
  };
}
