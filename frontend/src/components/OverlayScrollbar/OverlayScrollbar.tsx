import { CSSProperties, ReactElement, useMemo, type JSX } from 'react';
import TypicalOverlayScrollbar from 'components/TypicalOverlayScrollbar/TypicalOverlayScrollbar';
import VirtuosoOverlayScrollbar, {
	ScrollerProps,
} from 'components/VirtuosoOverlayScrollbar/VirtuosoOverlayScrollbar';
import { useIsDarkMode } from 'hooks/useDarkMode';
import { PartialOptions } from 'overlayscrollbars';

type Props = {
	children: ReactElement<ScrollerProps>;
	isVirtuoso?: boolean;
	style?: CSSProperties;
	options?: PartialOptions;
};

function OverlayScrollbar({
	children,
	isVirtuoso = false,
	style = {},
	options: customOptions = {},
}: Props): JSX.Element {
	const isDarkMode = useIsDarkMode();
	const options = useMemo(
		() =>
			({
				scrollbars: {
					autoHide: 'scroll',
					theme: isDarkMode ? 'os-theme-light' : 'os-theme-dark',
				},

				...(customOptions || {}),
			}) as PartialOptions,
		[customOptions, isDarkMode],
	);

	if (isVirtuoso) {
		return (
			<VirtuosoOverlayScrollbar style={style} options={options}>
				{children}
			</VirtuosoOverlayScrollbar>
		);
	}

	return (
		<TypicalOverlayScrollbar style={style} options={options}>
			{children}
		</TypicalOverlayScrollbar>
	);
}

export default OverlayScrollbar;
