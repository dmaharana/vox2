import React from 'react'

interface VoxLogoProps {
  className?: string
  size?: number
  showText?: boolean
  subtext?: string
}

export const VoxLogo: React.FC<VoxLogoProps> = ({
  className = '',
  size = 36,
  showText = false,
  subtext = 'AI Agent Studio',
}) => {
  return (
    <div className={`flex items-center gap-2.5 ${className}`}>
      <div
        style={{ width: size, height: size }}
        className="relative shrink-0 flex items-center justify-center rounded-xl overflow-hidden shadow-xs ring-1 ring-primary/20 bg-gradient-to-br from-cyan-500/20 via-indigo-500/20 to-purple-500/20"
      >
        <svg
          viewBox="0 0 128 128"
          width="100%"
          height="100%"
          className="w-full h-full"
        >
          <defs>
            <linearGradient id="voxGradInApp" x1="0%" y1="0%" x2="100%" y2="100%">
              <stop offset="0%" stopColor="#06b6d4" />
              <stop offset="50%" stopColor="#6366f1" />
              <stop offset="100%" stopColor="#a855f7" />
            </linearGradient>
            <linearGradient id="voxGrad2InApp" x1="0%" y1="100%" x2="100%" y2="0%">
              <stop offset="0%" stopColor="#38bdf8" />
              <stop offset="50%" stopColor="#818cf8" />
              <stop offset="100%" stopColor="#ec4899" />
            </linearGradient>
            <filter id="voxGlowInApp" x="-20%" y="-20%" width="140%" height="140%">
              <feGaussianBlur stdDeviation="2" result="blur" />
              <feComposite in="SourceGraphic" in2="blur" operator="over" />
            </filter>
          </defs>

          {/* Waveform pulse lines */}
          <path
            d="M 16 68 Q 28 68, 34 54 T 44 82 T 54 68 T 64 68"
            fill="none"
            stroke="url(#voxGradInApp)"
            strokeWidth="3.5"
            strokeLinecap="round"
            opacity="0.5"
          />
          <path
            d="M 64 68 Q 74 68, 84 54 T 94 82 T 104 68 T 112 68"
            fill="none"
            stroke="url(#voxGrad2InApp)"
            strokeWidth="3.5"
            strokeLinecap="round"
            opacity="0.5"
          />

          {/* Stylized Geometric V */}
          <path
            d="M 28 34 L 54 94 C 55.5 97.5 59.5 97.5 61 94 L 88 34 C 89.2 31.2 86.8 28 83.8 28 L 74 28 C 72.2 28 70.6 29.2 69.8 30.8 L 57.5 62 L 45.2 30.8 C 44.4 29.2 42.8 28 41 28 L 32.2 28 C 29.2 28 26.8 31.2 28 34 Z"
            fill="url(#voxGradInApp)"
            filter="url(#voxGlowInApp)"
          />

          {/* Futuristic Floating Superscript 2 */}
          <path
            d="M 80 24 C 80 18.5 84.5 14 90 14 C 95.5 14 100 18.5 100 24 C 100 28.5 96.5 32 92.5 35.5 L 83 44 L 102 44 L 102 49 L 78 49 L 78 44.5 L 90.5 33 C 93.5 30.2 95 27.5 95 24 C 95 21.2 92.8 19 90 19 C 87.2 19 85 21.2 85 24 L 80 24 Z"
            fill="url(#voxGrad2InApp)"
            filter="url(#voxGlowInApp)"
          />
        </svg>
      </div>

      {showText && (
        <div className="flex flex-col truncate leading-tight">
          <div className="flex items-center gap-1">
            <span className="text-sm font-extrabold tracking-tight bg-gradient-to-r from-cyan-400 via-indigo-400 to-purple-500 bg-clip-text text-transparent">
              VOX
            </span>
            <span className="text-xs font-black text-pink-500 -mt-1.5 font-mono">
              2
            </span>
          </div>
          {subtext && (
            <span className="text-[10px] text-muted-foreground font-mono truncate">
              {subtext}
            </span>
          )}
        </div>
      )}
    </div>
  )
}
