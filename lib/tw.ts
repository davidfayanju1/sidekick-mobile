import { create } from 'twrnc';

export const tw = create();

/** Strips whitespace so an rgb()/rgba() color string can be embedded in a twrnc arbitrary-value class, e.g. `bg-[${twColor(color)}]`. */
export function twColor(color: string): string {
  return color.replace(/\s+/g, '');
}
