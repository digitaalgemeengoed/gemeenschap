<?php
/**
 * Thema Digitaal GemeenGoed: een kindthema van BuddyX.
 *
 * Kleuren, voet en logo komen uit deze repo. Wat in de Customizer staat,
 * telt voor deze instellingen niet: een wijziging doe je hier, via GitHub.
 *
 * @package DigitaalGemeengoed
 */

defined( 'ABSPATH' ) || exit;

/** De kleuren uit het logo; tekst en knoppen halen WCAG AA (contrast ≥ 4,5). */
const DG_KLEUREN = array(
	'site_primary_color'                  => '#0066cc',
	'site_links_color'                    => '#0066cc',
	'site_links_focus_hover_color'        => '#003c7a',
	'site_title_hover_color'              => '#0066cc',
	'site_buttons_background_color'       => '#0066cc',
	'site_buttons_background_hover_color' => '#003c7a',
	'site_buttons_border_color'           => '#0066cc',
	'site_buttons_border_hover_color'     => '#003c7a',
	'site_buttons_text_color'             => '#ffffff',
	'site_buttons_text_hover_color'       => '#ffffff',
	'site_footer_links_hover_color'       => '#0066cc',
	'site_copyright_links_hover_color'    => '#0066cc',
);
// BuddyX leest zijn instellingen in één keer (get_theme_mods), dus de vaste
// waarden gaan over de opgeslagen instellingen heen, niet per instelling.
// De privacyverklaring staat al in het menu "Voet".
add_filter(
	'option_theme_mods_' . get_stylesheet(),
	static fn( $mods ) => array_merge( (array) $mods, DG_KLEUREN, array( 'site_custom_colors' => true, 'site_copyright_privacy_link' => 'off' ) )
);

// De voet: het menu "Voet" uit inhoud/menu.md en een regel over wie we zijn.
add_filter(
	'theme_mod_site_copyright_text',
	static function () {
		$menu = wp_get_nav_menu_object( 'Voet' )
			? wp_nav_menu(
				array(
					'menu'        => 'Voet',
					'depth'       => 1,
					'container'   => false,
					'menu_class'  => 'dg-voet',
					'echo'        => false,
					'fallback_cb' => false,
				)
			)
			: '';
		return $menu . '<p>Een netwerk van Digitaal GemeenGoed en NL-Dit. Open source, onder de EUPL.</p>';
	}
);

add_action(
	'wp_enqueue_scripts',
	static function () {
		wp_enqueue_style( 'digitaalgemeengoed', get_stylesheet_uri(), array(), wp_get_theme()->get( 'Version' ) );
	},
	20
);
