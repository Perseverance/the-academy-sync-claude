export function AcademyLogo({ className, fill }: { className?: string; fill?: string }) {
  return (
    <svg
      width="100"
      height="100"
      viewBox="0 0 100 100"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={className}
      aria-label="The Academy Logo"
    >
      <path d="M50 5L5 95H20L35 65H65L80 95H95L50 5ZM40 50L50 25L60 50H40Z" fill={fill || "hsl(var(--primary))"} />
    </svg>
  )
}
