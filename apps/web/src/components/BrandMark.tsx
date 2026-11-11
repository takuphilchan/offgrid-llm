/** In-app branding inherits semantic colors, including forced-color mode. */
export function BrandMark({size=28,className=''}:{size?:number;className?:string}) {
 return <svg className={`brand-symbol ${className}`} width={size} height={size} viewBox="0 0 64 64" fill="none" aria-hidden="true" focusable="false">
  <g stroke="currentColor" strokeWidth="3.25" strokeLinecap="round" strokeLinejoin="round">
   <path d="M32 14 15 22.5 32 31l17-8.5L32 14Z"/>
   <path d="m15 32 17 8.5L49 32"/>
   <path d="m15 41.5 17 8.5 17-8.5"/>
  </g>
 </svg>;
}
