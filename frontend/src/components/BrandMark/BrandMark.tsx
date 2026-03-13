import { CSSProperties } from 'react';
import { useTenant } from 'providers/Tenant';

import './BrandMark.styles.scss';

interface BrandMarkProps {
	size?: number | string;
	showProduct?: boolean;
	className?: string;
	style?: CSSProperties;
}

/**
 * White-label brand mark component.
 * Renders tenant logo (if provided) + wordmark text.
 * Falls back to text-only wordmark when no logo is configured.
 * Degrades gracefully — always shows something readable.
 */
function BrandMark({
	size = 24,
	showProduct = false,
	className = '',
	style,
}: BrandMarkProps): JSX.Element {
	const tenant = useTenant();
	const imgSize = typeof size === 'number' ? `${size}px` : size;

	return (
		<div
			className={`brand-mark ${className}`}
			style={{
				...style,
				'--brand-mark-size': imgSize,
			} as CSSProperties}
		>
			{tenant.logo ? (
				<img
					src={tenant.logo}
					alt={tenant.name}
					className="brand-mark-logo"
					onError={(e): void => {
						// If logo fails to load, hide it — wordmark text still visible
						(e.target as HTMLImageElement).style.display = 'none';
					}}
				/>
			) : null}
			<span className="brand-mark-name">{tenant.name}</span>
			{showProduct && tenant.productName && tenant.productName !== tenant.name && (
				<span className="brand-mark-product">{tenant.productName}</span>
			)}
		</div>
	);
}

export default BrandMark;
