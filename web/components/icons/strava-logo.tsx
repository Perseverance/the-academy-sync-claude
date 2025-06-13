import Image from "next/image"

export function StravaLogo({ className }: { className?: string }) {
  return (
    <div className={className} style={{ display: "inline-block", lineHeight: 0 }}>
      {" "}
      {/* Wrapper to help control layout if needed */}
      <Image
        src="/icons/strava-logo-circle.png"
        alt="Strava Logo"
        width={100} // Provide a base width, will be overridden by className if specified
        height={100} // Provide a base height
        className="h-full w-full" // Ensure image scales within the className dimensions
      />
    </div>
  )
}
