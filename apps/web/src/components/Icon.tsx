import { h, SVGAttributes } from 'preact';
import type { IconNode } from 'lucide';

interface IconProps extends SVGAttributes<SVGSVGElement> {
  icon: IconNode;
  size?: number | string;
  class?: string;
}

export function Icon({ icon, size = 16, class: className = '', ...props }: IconProps) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      class={className}
      {...props}
    >
      {icon.map(([tag, attrs], index) => {
        return h(tag, { key: index, ...attrs });
      })}
    </svg>
  );
}
