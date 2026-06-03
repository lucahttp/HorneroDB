/** @jsxImportSource react */

import { useId } from "react";

export const HorneroDbLogo = (props: React.SVGProps<SVGSVGElement>) => {
  const gradientId = useId();

  return (
    <svg
      viewBox="0 0 32 32"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      {...props}
    >
      <title>HorneroDB Logo</title>
      <rect width="32" height="32" rx="6" fill={`url(#${gradientId})`} />
      {/* Stylized "H" letter, evoking the HorneroDB brand */}
      <path
        d="M9 8.5V23.5M9 16H22.5M22.5 8.5V23.5"
        stroke="white"
        strokeWidth="2.4"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <defs>
        <linearGradient
          id={gradientId}
          x1="0"
          y1="0"
          x2="32"
          y2="32"
          gradientUnits="userSpaceOnUse"
        >
          <stop stopColor="#F59E0B" />
          <stop offset="1" stopColor="#B45309" />
        </linearGradient>
      </defs>
    </svg>
  );
};
