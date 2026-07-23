import * as AvatarPrimitive from '@rn-primitives/avatar';

import { tw } from '@/lib/tw';
import { useColorScheme } from '@/lib/useColorScheme';

function Avatar({ style, ...props }: AvatarPrimitive.RootProps) {
  return (
    <AvatarPrimitive.Root
      style={[tw`relative flex h-10 w-10 shrink-0 overflow-hidden rounded-full`, style]}
      {...props}
    />
  );
}

function AvatarImage({ style, ...props }: AvatarPrimitive.ImageProps) {
  return <AvatarPrimitive.Image style={[tw`aspect-square h-full w-full`, style]} {...props} />;
}

function AvatarFallback({ style, ...props }: AvatarPrimitive.FallbackProps) {
  const { colors } = useColorScheme();
  return (
    <AvatarPrimitive.Fallback
      style={[
        tw`h-full w-full items-center justify-center rounded-full`,
        { backgroundColor: colors.muted },
        style,
      ]}
      {...props}
    />
  );
}

export { Avatar, AvatarFallback, AvatarImage };
