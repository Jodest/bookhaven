import * as React from 'react';

export interface CheckboxProps
  extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label: string;
  count?: number;
}

const Checkbox = React.forwardRef<HTMLInputElement, CheckboxProps>(
  ({ className, label, count, ...props }, ref) => {
    return (
      <label className="flex cursor-pointer items-center gap-2">
        <input
          type="checkbox"
          className={
            'h-4 w-4 rounded border-input bg-background text-primary focus:ring-2 focus:ring-ring focus:ring-offset-2' +
            (className ? ` ${className}` : '')
          }
          ref={ref}
          {...props}
        />
        <span className="text-sm">
          {label}
          {count !== undefined && (
            <span className="ml-1 text-muted-foreground">({count})</span>
          )}
        </span>
      </label>
    );
  }
);
Checkbox.displayName = 'Checkbox';

export { Checkbox };
