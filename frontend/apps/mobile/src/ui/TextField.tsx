import { color } from '@swp/design-tokens';
// Matches .pen design: label + bordered box with icon slot.
// Design: TextField `Box` = fill=surface, cornerRadius=8, stroke=border, padding=[13,14]
// Icon on right side with fill=text-3.
import { TextInput, type TextInputProps, View } from 'react-native';
import { Text } from './Text';

export interface FieldIcon {
  library: 'lucide' | 'feather' | 'MaterialSymbols';
  name: string;
  /** Default 16 */
  size?: number;
}

export function TextField({
  label,
  value,
  onChangeText,
  placeholder,
  secureTextEntry,
  icon,
  error,
  invalid,
  keyboardType,
  autoCapitalize,
  autoCorrect,
  multiline,
  testID,
  children,
}: TextInputProps & {
  label?: string;
  error?: string;
  invalid?: boolean;
  icon?: FieldIcon;
  children?: React.ReactNode;
}) {
  return (
    <View className="gap-1.5">
      {label ? (
        <Text variant="label" weight="semibold" className="text-text-2">
          {label}
        </Text>
      ) : null}
      <View
        className={`flex-row items-center justify-between rounded-input border bg-surface px-3.5 py-[13px] ${
          invalid ? 'border-bad-text' : 'border-border'
        }`}
      >
        <TextInput
          testID={testID}
          value={value}
          onChangeText={onChangeText}
          placeholder={placeholder}
          placeholderTextColor={color.text3}
          secureTextEntry={secureTextEntry}
          keyboardType={keyboardType}
          autoCapitalize={autoCapitalize}
          autoCorrect={autoCorrect}
          multiline={multiline}
          className="flex-1 text-sm text-text"
        />
        {children ?? null}
      </View>
      {error ? (
        <Text variant="caption" className="text-bad-text">
          {error}
        </Text>
      ) : null}
    </View>
  );
}
