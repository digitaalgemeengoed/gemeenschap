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
// De privacyverklaring staat al in het menu "Voet". Geen zijbalken: de pagina's
// gebruiken de hele breedte (anders toont BuddyX de standaardwidgets van een
// nieuwe installatie, zoals Recent Posts, naast de opening).
add_filter(
	'option_theme_mods_' . get_stylesheet(),
	static fn( $mods ) => array_merge(
		(array) $mods,
		DG_KLEUREN,
		array(
			'site_custom_colors'          => true,
			// Breed genoeg voor logo, ondertitel en het hele hoofdmenu (anders "More").
			'site_container_width'        => '1380px',
			'site_copyright_privacy_link' => 'off',
			'sidebar_option'              => 'none',
			'single_post_sidebar_option'  => 'none',
			'buddypress_sidebar_option'   => 'none',
			'buddypress_members_sidebar_option' => 'none',
			'buddypress_groups_sidebar_option'  => 'none',
			'bbpress_sidebar_option'      => 'none',
		)
	)
);

// De voet: het menu "Voet" uit inhoud/menu.md en een regel over wie we zijn.
add_filter( 'theme_mod_site_copyright_text', 'dg_voet' );

/**
 * De voet uit het menu "Voet" (inhoud/menu.md): links het logo met een korte
 * omschrijving, dan een kolom per punt zonder link met de punten eronder, en
 * onderaan de punten met een link.
 */
function dg_voet(): string {
	$items    = wp_get_nav_menu_object( 'Voet' ) ? (array) wp_get_nav_menu_items( 'Voet' ) : array();
	$kolommen = array();
	$onder    = array();
	foreach ( $items as $item ) {
		if ( (int) $item->menu_item_parent ) {
			$kolommen[ (int) $item->menu_item_parent ]['links'][] = $item;
		} elseif ( '#' === $item->url ) {
			$kolommen[ (int) $item->ID ]['titel'] = $item->title;
		} else {
			$onder[] = $item;
		}
	}
	$link = static fn( $item ) => '<a href="' . esc_url( $item->url ) . '">' . esc_html( $item->title ) . '</a>';
	$html = '<div class="dg-voet-boven"><div class="dg-voet-merk"><img src="' . esc_url( get_stylesheet_directory_uri() . '/assets/uil.png' ) . '" alt="" width="40" height="78"><div><strong>Digitaal<br>GemeenGoed</strong><span>Samen grenzeloos sterk</span></div>'
		. '<p>Een community voor kennisdeling, gezamenlijke ontwikkeling en duurzaam beheer van open source oplossingen voor de overheid.</p></div>';
	foreach ( $kolommen as $kolom ) {
		if ( empty( $kolom['titel'] ) || empty( $kolom['links'] ) ) {
			continue;
		}
		$html .= '<div class="dg-voet-kolom"><h3>' . esc_html( $kolom['titel'] ) . '</h3><ul>' . implode( '', array_map( static fn( $i ) => '<li>' . $link( $i ) . '</li>', $kolom['links'] ) ) . '</ul></div>';
	}
	$html .= '</div><div class="dg-voet-onder"><span>© ' . esc_html( wp_date( 'Y' ) ) . ' Stichting Digitaal GemeenGoed · in samenwerking met NL-Dit · open source onder de EUPL</span>'
		. ( $onder ? '<span class="dg-voet-links">' . implode( '', array_map( $link, $onder ) ) . '</span>' : '' ) . '</div>';
	return $html;
}

add_action(
	'wp_enqueue_scripts',
	static function () {
		wp_enqueue_style( 'digitaalgemeengoed', get_stylesheet_uri(), array(), wp_get_theme()->get( 'Version' ) );
	},
	20
);

// "Log in" in de kop heet bij ons "Aanmelden" (het ontwerp; zo heet het ook in Keycloak).
add_filter(
	'gettext_buddyx',
	static fn( $vertaling, $tekst ) => 'Log in' === $tekst ? 'Aanmelden' : $vertaling,
	10,
	2
);

// De zwarte WordPress-werkbalk alleen voor wie pagina's of berichten bewerkt;
// bezoekers en gewone leden hebben de kop van de site (BuddyPress toont hem
// anders ook aan wie niet is ingelogd).
add_filter( 'show_admin_bar', static fn( $tonen ) => $tonen && is_user_logged_in() && current_user_can( 'edit_posts' ), 99 );

// Een pagina die begint met een opening (::: held in inhoud/): geen titelbalk van
// BuddyX met kruimelpad, want de opening heeft de titel al.
function dg_heeft_held(): bool {
	if ( ! is_page() ) {
		return false;
	}
	$post = get_queried_object();
	return $post instanceof WP_Post && str_contains( $post->post_content, 'dg-held' );
}
add_action(
	'wp',
	static function () {
		if ( dg_heeft_held() ) {
			remove_action( 'buddyx_sub_header', 'buddyx_sub_header' );
		}
	}
);
add_filter( 'body_class', static fn( $klassen ) => dg_heeft_held() ? array_merge( $klassen, array( 'dg-met-held' ) ) : $klassen );
