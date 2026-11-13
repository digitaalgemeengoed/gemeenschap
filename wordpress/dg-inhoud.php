<?php
/**
 * Plugin Name: Digitaal GemeenGoed — pagina's uit GitHub
 * Description: De pagina's in inhoud/paginas/ van de repo digitaalgemeengoed/gemeenschap worden WordPress-pagina's. Die pagina's wijzig je via GitHub; in WordPress staan ze op slot.
 *
 * Een pagina is een Markdown-bestand met bovenaan een kopje tussen twee
 * regels "---" (zie inhoud/README.md):
 *
 *   ---
 *   titel: Privacyverklaring
 *   adres: privacy
 *   ---
 *   Tekst in Markdown …
 *
 * De uitrol (server/uitrol.sh) draait na elke wijziging op main:
 *
 *   wp dg inhoud /inhoud
 *
 * Opnieuw draaien kan zonder schade: een pagina die niet veranderd is, blijft
 * staan zoals hij is. Een pagina die uit de repo verdwijnt, wordt een concept.
 *
 * @package DigitaalGemeengoed
 */

defined( 'ABSPATH' ) || exit;

/** Ophogen als de omzetting verandert: dan worden alle pagina's opnieuw opgeslagen. */
const DG_INHOUD_VERSIE = 5;

// ---------------------------------------------------------------------------
// Op slot: pagina's en afbeeldingen uit de repo wijzig je via GitHub
// ---------------------------------------------------------------------------

/** Het pad in de repo (paginas/…md of media/…) als dit bericht uit de repo komt. */
function dg_inhoud_bron( $post ): string {
	$id = $post instanceof WP_Post ? $post->ID : (int) $post;
	return $id ? (string) get_post_meta( $id, '_dg_bron', true ) : '';
}

/** De link naar het bestand op GitHub. */
function dg_inhoud_github( string $bron ): string {
	$basis = rtrim( (string) ( getenv( 'DG_INHOUD_REPO' ) ?: 'https://github.com/digitaalgemeengoed/gemeenschap/blob/main/inhoud' ), '/' );
	return $basis . '/' . implode( '/', array_map( 'rawurlencode', explode( '/', $bron ) ) );
}

add_filter(
	'map_meta_cap',
	static function ( $caps, $cap, $user_id, $args ) {
		$per_bericht = array( 'edit_post', 'delete_post', 'publish_post', 'edit_page', 'delete_page' );
		if ( in_array( $cap, $per_bericht, true ) && ! empty( $args[0] ) && dg_inhoud_bron( (int) $args[0] ) ) {
			return array( 'do_not_allow' );
		}
		return $caps;
	},
	10,
	4
);

// In het overzicht van pagina's: "uit GitHub" en een link naar het bestand.
add_filter(
	'display_post_states',
	static function ( $staten, $post ) {
		if ( dg_inhoud_bron( $post ) ) {
			$staten['dg_inhoud'] = 'Uit GitHub';
		}
		return $staten;
	},
	10,
	2
);
add_filter(
	'page_row_actions',
	static function ( $acties, $post ) {
		$bron = dg_inhoud_bron( $post );
		if ( $bron ) {
			$acties['dg_github'] = '<a href="' . esc_url( dg_inhoud_github( $bron ) ) . '" target="_blank" rel="noopener">Bewerken op GitHub</a>';
		}
		return $acties;
	},
	10,
	2
);

// In de adminbalk op de pagina zelf, voor wie pagina's mag bewerken.
add_action(
	'admin_bar_menu',
	static function ( $balk ) {
		if ( is_admin() || ! is_singular() || ! current_user_can( 'edit_pages' ) ) {
			return;
		}
		$bron = dg_inhoud_bron( get_queried_object_id() );
		if ( $bron ) {
			$balk->add_node(
				array(
					'id'    => 'dg-github',
					'title' => 'Bewerken op GitHub',
					'href'  => dg_inhoud_github( $bron ),
					'meta'  => array( 'target' => '_blank', 'rel' => 'noopener' ),
				)
			);
		}
	},
	81
);

// ---------------------------------------------------------------------------
// Markdown lezen
// ---------------------------------------------------------------------------

/**
 * Splitst een bestand in kopje (sleutel: waarde) en tekst.
 *
 * @return array{0: array<string,string>, 1: string}
 */
function dg_inhoud_lees( string $tekst ): array {
	$tekst = str_replace( array( "\r\n", "\r" ), "\n", $tekst );
	$tekst = preg_replace( '/^\xEF\xBB\xBF/', '', $tekst );
	$kop   = array();
	if ( preg_match( '/\A---\n(.*?)\n---\n?(.*)\z/s', $tekst, $m ) ) {
		foreach ( explode( "\n", $m[1] ) as $regel ) {
			if ( '' === trim( $regel ) || str_starts_with( ltrim( $regel ), '#' ) ) {
				continue;
			}
			if ( ! preg_match( '/^([a-z_]+):\s*(.*)$/', $regel, $k ) ) {
				throw new RuntimeException( "regel in het kopje niet begrepen: \"$regel\"" );
			}
			$kop[ $k[1] ] = trim( $k[2], " \t\"'" );
		}
		$tekst = $m[2];
	}
	return array( $kop, $tekst );
}

/** Tekst binnen een blok: HTML ontsnapt, dan vet, cursief, code en links. */
function dg_inhoud_regel( string $tekst, callable $link ): string {
	$codes = array();
	$tekst = preg_replace_callback(
		'/`([^`]+)`/',
		static function ( $m ) use ( &$codes ) {
			$codes[] = '<code>' . esc_html( $m[1] ) . '</code>';
			return "\x00" . ( count( $codes ) - 1 ) . "\x00";
		},
		$tekst
	);
	$tekst = esc_html( $tekst );
	$tekst = preg_replace_callback(
		'/\[([^\]]+)\]\(([^)\s]+)\)/',
		static function ( $m ) use ( $link ) {
			$url    = $link( html_entity_decode( $m[2], ENT_QUOTES ) );
			$extern = preg_match( '#^https?://#', $url ) && ! str_starts_with( $url, home_url() );
			return '<a href="' . esc_url( $url ) . '"' . ( $extern ? ' rel="noopener"' : '' ) . '>' . $m[1] . '</a>';
		},
		$tekst
	);
	$tekst = preg_replace( '/\*\*(.+?)\*\*/', '<strong>$1</strong>', $tekst );
	$tekst = preg_replace( '/(?<![\w*])\*(?!\s)(.+?)(?<!\s)\*(?![\w*])/', '<em>$1</em>', $tekst );
	$tekst = preg_replace( '/(?<![\w_])_(?!\s)(.+?)(?<!\s)_(?![\w_])/', '<em>$1</em>', $tekst );
	return preg_replace_callback( "/\x00(\d+)\x00/", static fn( $m ) => $codes[ (int) $m[1] ], $tekst );
}

/** Een blok in de opmaak van de blokeditor. */
function dg_blok( string $naam, string $html, array $attributen = array() ): string {
	$attr = $attributen ? ' ' . wp_json_encode( $attributen, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE ) : '';
	return "<!-- wp:$naam$attr -->\n$html\n<!-- /wp:$naam -->";
}

/** De vakken die een pagina kan gebruiken (::: naam … :::), zie inhoud/README.md. */
const DG_VAKKEN = array( 'held', 'kaarten', 'stappen', 'kenmerken', 'projecten', 'kader', 'band', 'feiten' );

/** Vakken waarin elke ### kop een kaart begint. */
const DG_KAARTVAKKEN = array( 'kaarten', 'stappen', 'kenmerken', 'projecten' );

/** De iconen in het thema (wordpress/thema/…/assets/iconen/<naam>.svg). */
const DG_ICONEN = array( 'open', 'lagen', 'mensen', 'raket', 'plant', 'document', 'mail', 'werkplek', 'forum', 'boek', 'schild', 'stad', 'puzzel', 'hand', 'kompas', 'code', 'huis', 'kalender', 'instellingen' );

/** Regels "sleutel: waarde" bovenaan een vak of kaart. */
const DG_KENMERKEN = array( 'label', 'beeld', 'icoon', 'link', 'soort', 'doel', 'toegezegd', 'deelnemers', 'campagne', 'einddatum' );

/**
 * Haalt de regels "sleutel: waarde" (label, beeld, icoon, link, soort) uit een
 * stuk Markdown. Label mag vaker voorkomen.
 *
 * @return array{0: array<string, list<string>>, 1: string}
 */
function dg_inhoud_kenmerken( string $md ): array {
	$kenmerken = array();
	$rest      = array();
	foreach ( explode( "\n", $md ) as $regel ) {
		if ( preg_match( '/^\s*(' . implode( '|', DG_KENMERKEN ) . '):\s*(.+?)\s*$/', $regel, $m ) ) {
			$kenmerken[ $m[1] ][] = $m[2];
		} else {
			$rest[] = $regel;
		}
	}
	return array( $kenmerken, implode( "\n", $rest ) );
}

/** Een label als klasse: "Common Ground" → common-ground. */
function dg_inhoud_slug( string $tekst ): string {
	return trim( (string) preg_replace( '/[^a-z0-9]+/', '-', strtolower( remove_accents( $tekst ) ) ), '-' );
}

/** Het icoon als leeg element; het thema tekent het (style.css, .dg-icoon-<naam>). */
function dg_inhoud_icoon( string $naam ): string {
	return '<span class="dg-icoon dg-icoon-' . esc_attr( $naam ) . '" aria-hidden="true"></span>';
}

/**
 * Een vak: een groep blokken met een eigen vorm (in het thema, style.css).
 *
 *   held       de opening van een pagina: grote kop, linten (label:), inleiding,
 *              knoppen, en rechts een foto (beeld:)
 *   kaarten    elke ### kop met de tekst eronder wordt een kaart, naast elkaar
 *   stappen    als kaarten, maar genummerd: een proces in stappen
 *   kenmerken  een strook met icoon, kop en een korte regel per kenmerk
 *   projecten  kaarten met icoon of beeld, label en een pijl naar de pagina;
 *              een kaart met "soort: oproep" wordt een blauw vlak ernaast
 *   kader      tekst in een gekleurd vlak
 */
function dg_inhoud_vak( string $soort, string $md, callable $link, callable $beeld ): string {
	if ( in_array( $soort, DG_KAARTVAKKEN, true ) ) {
		$delen   = preg_split( '/^###\s+/m', "\n" . $md );
		$kaarten = array();
		foreach ( array_slice( $delen, 1 ) as $deel ) {
			[ $kop, $rest ]       = array_pad( explode( "\n", $deel, 2 ), 2, '' );
			[ $kenmerk, $rest ]   = dg_inhoud_kenmerken( $rest );
			$klassen              = array( 'dg-kaart' );
			$binnen               = '';
			if ( ! empty( $kenmerk['beeld'] ) ) {
				[ $id, $url ] = $beeld( $kenmerk['beeld'][0] );
				$binnen      .= dg_blok( 'image', '<figure class="wp-block-image size-medium dg-kaart-beeld"><img src="' . esc_url( $url ) . '" alt="" class="wp-image-' . $id . '"/></figure>', array( 'id' => $id, 'sizeSlug' => 'medium', 'className' => 'dg-kaart-beeld' ) ) . "\n\n";
				$klassen[]    = 'heeft-beeld';
			}
			foreach ( $kenmerk['label'] ?? array() as $label ) {
				$binnen .= dg_blok( 'paragraph', '<p class="dg-label dg-label-' . dg_inhoud_slug( $label ) . '">' . esc_html( $label ) . '</p>', array( 'className' => 'dg-label dg-label-' . dg_inhoud_slug( $label ) ) ) . "\n\n";
			}
			$icoon   = ! empty( $kenmerk['icoon'] ) ? dg_inhoud_icoon( $kenmerk['icoon'][0] ) : '';
			$binnen .= dg_blok( 'heading', '<h3 class="wp-block-heading">' . $icoon . dg_inhoud_regel( trim( $kop ), $link ) . '</h3>', array( 'level' => 3 ) ) . "\n\n" . dg_inhoud_blokken( $rest, $link, $beeld );
			if ( ! empty( $kenmerk['campagne'] ) && ! empty( $kenmerk['doel'] ) ) {
				// Een campagne met toezeggingen op de site (dg-crowdfunding.php).
				$code    = '[dg_campagne id=' . sanitize_key( $kenmerk['campagne'][0] ) . ' doel=' . (int) preg_replace( '/\D/', '', $kenmerk['doel'][0] ) . ( ! empty( $kenmerk['einddatum'] ) ? ' einddatum=' . preg_replace( '/[^0-9-]/', '', $kenmerk['einddatum'][0] ) : '' ) . ']';
				$binnen .= "\n\n" . dg_blok( 'shortcode', $code );
			} elseif ( ! empty( $kenmerk['doel'] ) ) {
				$binnen .= "\n\n" . dg_inhoud_campagne( $kenmerk );
			}
			if ( ! empty( $kenmerk['link'] ) && ! empty( $kenmerk['campagne'] ) ) {
				// Bij een campagne staat er een formulier op de kaart: geen klikbare kaart, maar een gewone link.
				$binnen .= "\n\n" . dg_blok( 'paragraph', '<p class="dg-kaart-meer"><a href="' . esc_url( $link( $kenmerk['link'][0] ) ) . '">Meer over het project →</a></p>', array( 'className' => 'dg-kaart-meer' ) );
			} elseif ( ! empty( $kenmerk['link'] ) ) {
				$binnen   .= "\n\n" . dg_blok( 'paragraph', '<p class="dg-kaart-link"><a href="' . esc_url( $link( $kenmerk['link'][0] ) ) . '"><span class="screen-reader-text">' . esc_html( trim( $kop ) ) . '</span></a></p>', array( 'className' => 'dg-kaart-link' ) );
				$klassen[] = 'heeft-link';
			}
			if ( $icoon ) {
				$klassen[] = 'heeft-icoon';
			}
			foreach ( $kenmerk['soort'] ?? array() as $extra ) {
				$klassen[] = 'dg-kaart-' . dg_inhoud_slug( $extra );
			}
			$klasse    = implode( ' ', $klassen );
			$kaarten[] = dg_blok( 'group', '<div class="wp-block-group ' . $klasse . '">' . $binnen . '</div>', array( 'className' => $klasse, 'layout' => array( 'type' => 'default' ) ) );
		}
		$klasse = 'dg-' . $soort;
		return dg_blok( 'group', '<div class="wp-block-group ' . $klasse . '">' . implode( "\n\n", $kaarten ) . '</div>', array( 'className' => $klasse, 'layout' => array( 'type' => 'grid', 'minimumColumnWidth' => '15rem' ) ) );
	}
	$klasse = 'dg-' . $soort;
	if ( 'feiten' === $soort ) {
		// Regels "Sleutel: waarde" als een tabel met twee kolommen.
		$rijen = '';
		foreach ( explode( "\n", $md ) as $regel ) {
			if ( preg_match( '/^\s*([^:]{1,40}):\s*(.+?)\s*$/', $regel, $m ) ) {
				$rijen .= '<tr><th>' . dg_inhoud_regel( $m[1], $link ) . '</th><td>' . dg_inhoud_regel( $m[2], $link ) . '</td></tr>';
			}
		}
		return dg_blok( 'table', '<figure class="wp-block-table dg-feiten"><table><tbody>' . $rijen . '</tbody></table></figure>', array( 'hasFixedLayout' => false, 'className' => 'dg-feiten' ) );
	}
	if ( 'held' === $soort ) {
		[ $kenmerk, $md ] = dg_inhoud_kenmerken( $md );
		$binnen           = dg_inhoud_blokken( $md, $link, $beeld );
		if ( ! empty( $kenmerk['label'] ) ) {
			$linten = '';
			foreach ( $kenmerk['label'] as $nr => $label ) {
				$linten .= dg_blok( 'paragraph', '<p class="dg-lint dg-lint-' . ( $nr % 2 ? 'groen' : 'blauw' ) . '">' . esc_html( $label ) . '</p>', array( 'className' => 'dg-lint dg-lint-' . ( $nr % 2 ? 'groen' : 'blauw' ) ) ) . "\n\n";
			}
			// De linten direct onder de eerste kop.
			$na     = strpos( $binnen, '<!-- /wp:heading -->' );
			$binnen = false === $na ? $linten . $binnen : substr( $binnen, 0, $na + 20 ) . "\n\n" . $linten . substr( $binnen, $na + 20 );
		}
		$tekst = dg_blok( 'group', '<div class="wp-block-group dg-held-tekst">' . $binnen . '</div>', array( 'className' => 'dg-held-tekst', 'layout' => array( 'type' => 'default' ) ) );
		$foto  = '';
		if ( ! empty( $kenmerk['beeld'] ) ) {
			[ $id, $url ] = $beeld( $kenmerk['beeld'][0] );
			$foto         = "\n\n" . dg_blok( 'image', '<figure class="wp-block-image size-full dg-held-beeld"><img src="' . esc_url( $url ) . '" alt="" class="wp-image-' . $id . '"/></figure>', array( 'id' => $id, 'sizeSlug' => 'full', 'className' => 'dg-held-beeld' ) );
			$klasse      .= ' heeft-beeld';
		}
		return dg_blok( 'group', '<div class="wp-block-group ' . $klasse . '">' . $tekst . $foto . '</div>', array( 'className' => $klasse, 'align' => 'full', 'layout' => array( 'type' => 'default' ) ) );
	}
	return dg_blok( 'group', '<div class="wp-block-group ' . $klasse . '">' . dg_inhoud_blokken( $md, $link, $beeld ) . '</div>', array( 'className' => $klasse, 'layout' => array( 'type' => 'constrained' ) ) );
}

/**
 * Een voortgangsbalk voor crowdfunding: doel en toegezegd in euro's, en
 * eventueel het aantal deelnemende organisaties.
 */
function dg_inhoud_campagne( array $kenmerk ): string {
	$getal     = static fn( string $tekst ): int => (int) preg_replace( '/\D/', '', $tekst );
	$euro      = static fn( int $bedrag ): string => '€ ' . number_format( $bedrag, 0, ',', '.' );
	$doel      = $getal( $kenmerk['doel'][0] );
	$toegezegd = $getal( $kenmerk['toegezegd'][0] ?? '0' );
	$procent   = $doel ? min( 100, (int) round( 100 * $toegezegd / $doel ) ) : 0;
	$deel      = ! empty( $kenmerk['deelnemers'] ) ? ' · ' . esc_html( $kenmerk['deelnemers'][0] ) : '';
	$html      = '<p class="dg-campagne"><span class="dg-balk" role="img" aria-label="' . $procent . ' procent toegezegd"><span class="dg-balk-vul" style="width:' . $procent . '%"></span></span>'
		. '<span class="dg-bedragen"><strong>' . $euro( $toegezegd ) . '</strong> toegezegd van ' . $euro( $doel ) . $deel . '</span></p>';
	return dg_blok( 'paragraph', $html, array( 'className' => 'dg-campagne' ) );
}

/** Fouten in de vakken van een pagina: onbekende naam, niet afgesloten, kaarten zonder ### kop. */
function dg_inhoud_controleer_vakken( string $md ): array {
	$fouten = array();
	$open   = null;
	foreach ( explode( "\n", $md ) as $nr => $regel ) {
		if ( preg_match( '/^\s*:::\s*([a-z]*)\s*$/', $regel, $m ) ) {
			if ( null === $open && '' === $m[1] ) {
				$fouten[] = 'regel ' . ( $nr + 1 ) . ': ::: sluit een vak dat niet geopend is.';
			} elseif ( null === $open ) {
				if ( ! in_array( $m[1], DG_VAKKEN, true ) ) {
					$fouten[] = 'regel ' . ( $nr + 1 ) . ": onbekend vak \"{$m[1]}\" (kies uit " . implode( ', ', DG_VAKKEN ) . ').';
				}
				$open = array( $m[1], $nr + 1, false );
			} elseif ( '' === $m[1] ) {
				if ( in_array( $open[0], DG_KAARTVAKKEN, true ) && ! $open[2] ) {
					$fouten[] = "regel {$open[1]}: in een vak \"{$open[0]}\" begint elke kaart met een kop ###.";
				}
				$open = null;
			} else {
				$fouten[] = 'regel ' . ( $nr + 1 ) . ': een vak in een vak kan niet; sluit eerst af met :::.';
			}
		} elseif ( null !== $open && preg_match( '/^###\s+/', $regel ) ) {
			$open[2] = true;
		} elseif ( null !== $open && preg_match( '/^\s*einddatum:\s*(.*?)\s*$/', $regel, $m ) && ! preg_match( '/^\d{4}-\d{2}-\d{2}$/', $m[1] ) ) {
			$fouten[] = 'regel ' . ( $nr + 1 ) . ": einddatum schrijf je als jjjj-mm-dd, bijvoorbeeld 2027-03-01.";
		} elseif ( null !== $open && preg_match( '/^\s*campagne:\s*(.*?)\s*$/', $regel, $m ) && ! preg_match( '/^[a-z0-9]+(-[a-z0-9]+)*$/', $m[1] ) ) {
			$fouten[] = 'regel ' . ( $nr + 1 ) . ": campagne mag alleen kleine letters, cijfers en streepjes bevatten.";
		} elseif ( null !== $open && preg_match( '/^\s*icoon:\s*(\S+)/', $regel, $m ) && ! in_array( $m[1], DG_ICONEN, true ) ) {
			$fouten[] = 'regel ' . ( $nr + 1 ) . ": onbekend icoon \"{$m[1]}\" (kies uit " . implode( ', ', DG_ICONEN ) . ').';
		}
	}
	if ( null !== $open ) {
		$fouten[] = "regel {$open[1]}: het vak \"{$open[0]}\" wordt niet afgesloten met :::.";
	}
	return $fouten;
}

/**
 * Markdown naar blokken. Kent: kopjes (## tot ####), alinea's, lijsten (- en
 * 1.), citaten (>), tabellen (| a | b |), een lijn (---), een afbeelding of
 * een bestaande shortcode (zoals [dg_lid_worden]) op een eigen regel, en binnen de tekst **vet**, *cursief*,
 * `code` en [links](adres). HTML in de tekst wordt tekst, geen opmaak.
 *
 * @param callable $link  Maakt van een link in de tekst een adres (andere pagina's: naam.md).
 * @param callable $beeld Geeft voor media/… array( id, url ) van de afbeelding.
 */
function dg_inhoud_blokken( string $md, callable $link, callable $beeld ): string {
	$regels = explode( "\n", $md );
	$blokken = array();
	$alinea  = array();
	$n       = count( $regels );
	$sluit   = static function () use ( &$alinea, &$blokken, $link ) {
		if ( $alinea ) {
			$blokken[] = dg_blok( 'paragraph', '<p>' . dg_inhoud_regel( implode( ' ', $alinea ), $link ) . '</p>' );
			$alinea    = array();
		}
	};
	for ( $i = 0; $i < $n; $i++ ) {
		$r = rtrim( $regels[ $i ] );
		$t = ltrim( $r );
		if ( '' === $t ) {
			$sluit();
			continue;
		}
		if ( preg_match( '/^:::\s*([a-z]+)\s*$/', $t, $m ) ) {
			$sluit();
			$binnen = array();
			for ( ++$i; $i < $n && ! preg_match( '/^\s*:::\s*$/', $regels[ $i ] ); $i++ ) {
				$binnen[] = $regels[ $i ];
			}
			$blokken[] = dg_inhoud_vak( $m[1], implode( "\n", $binnen ), $link, $beeld );
			continue;
		}
		if ( preg_match( '/^(#{1,4})\s+(.+?)\s*#*$/', $t, $m ) ) {
			$sluit();
			$niveau    = max( 2, strlen( $m[1] ) );
			$blokken[] = dg_blok( 'heading', "<h$niveau class=\"wp-block-heading\">" . dg_inhoud_regel( $m[2], $link ) . "</h$niveau>", 2 === $niveau ? array() : array( 'level' => $niveau ) );
			continue;
		}
		if ( preg_match( '/^\[\[([^\]]+)\]\]\(([^)\s]+)\)$/', $t ) ) {
			// Knoppen: [[Tekst]](adres), één of meer onder elkaar.
			$sluit();
			$knoppen = array();
			while ( $i < $n && preg_match( '/^\s*\[\[([^\]]+)\]\]\(([^)\s]+)\)\s*$/', $regels[ $i ], $k ) ) {
				$tweede    = $knoppen ? ' is-style-outline' : '';
				$knoppen[] = dg_blok( 'button', '<div class="wp-block-button' . $tweede . '"><a class="wp-block-button__link wp-element-button" href="' . esc_url( $link( $k[2] ) ) . '">' . esc_html( $k[1] ) . '</a></div>', $tweede ? array( 'className' => 'is-style-outline' ) : array() );
				++$i;
			}
			--$i;
			$blokken[] = dg_blok( 'buttons', '<div class="wp-block-buttons">' . implode( "\n", $knoppen ) . '</div>' );
			continue;
		}
		if ( preg_match( '/^(-{3,}|\*{3,})$/', $t ) ) {
			$sluit();
			$blokken[] = dg_blok( 'separator', '<hr class="wp-block-separator has-alpha-channel-opacity"/>' );
			continue;
		}
		if ( preg_match( '/^!\[([^\]]*)\]\(([^)\s]+)\)$/', $t, $m ) ) {
			$sluit();
			[ $id, $url ] = $beeld( $m[2] );
			$blokken[]    = dg_blok( 'image', '<figure class="wp-block-image size-large"><img src="' . esc_url( $url ) . '" alt="' . esc_attr( $m[1] ) . '" class="wp-image-' . $id . '"/></figure>', array( 'id' => $id, 'sizeSlug' => 'large' ) );
			continue;
		}
		if ( preg_match( '/^\[([a-z][a-z0-9_]*)(\s[^\]]*)?\]$/', $t, $m ) && shortcode_exists( $m[1] ) ) {
			$sluit();
			$blokken[] = dg_blok( 'shortcode', $t );
			continue;
		}
		if ( preg_match( '/^([-*]|\d+[.)])\s+/', $t, $m ) ) {
			$sluit();
			$geordend = ! in_array( $m[1], array( '-', '*' ), true );
			$items    = array();
			while ( $i < $n && preg_match( $geordend ? '/^\s*\d+[.)]\s+(.*)$/' : '/^\s*[-*]\s+(.*)$/', $regels[ $i ], $li ) ) {
				$tekst = $li[1];
				// Een vervolgregel (ingesprongen) hoort bij hetzelfde punt.
				while ( $i + 1 < $n && preg_match( '/^\s{2,}(\S.*)$/', $regels[ $i + 1 ], $verder ) && ! preg_match( '/^\s*([-*]|\d+[.)])\s+/', $regels[ $i + 1 ] ) ) {
					$tekst .= ' ' . $verder[1];
					++$i;
				}
				$items[] = dg_blok( 'list-item', '<li>' . dg_inhoud_regel( $tekst, $link ) . '</li>' );
				++$i;
			}
			--$i;
			$tag       = $geordend ? 'ol' : 'ul';
			$blokken[] = dg_blok( 'list', "<$tag class=\"wp-block-list\">" . implode( '', $items ) . "</$tag>", $geordend ? array( 'ordered' => true ) : array() );
			continue;
		}
		if ( str_starts_with( $t, '>' ) ) {
			$sluit();
			$citaat = array();
			while ( $i < $n && str_starts_with( ltrim( $regels[ $i ] ), '>' ) ) {
				$citaat[] = ltrim( substr( ltrim( $regels[ $i ] ), 1 ) );
				++$i;
			}
			--$i;
			$alineas = array_filter( array_map( 'trim', explode( "\n\n", implode( "\n", $citaat ) ) ) );
			$binnen  = implode( '', array_map( static fn( $a ) => dg_blok( 'paragraph', '<p>' . dg_inhoud_regel( str_replace( "\n", ' ', $a ), $link ) . '</p>' ), $alineas ) );
			$blokken[] = dg_blok( 'quote', '<blockquote class="wp-block-quote">' . $binnen . '</blockquote>' );
			continue;
		}
		if ( str_starts_with( $t, '|' ) && $i + 1 < $n && preg_match( '/^\s*\|?\s*:?-{3,}/', $regels[ $i + 1 ] ) ) {
			$sluit();
			$cellen = static fn( string $rij ): array => array_map( 'trim', explode( '|', trim( trim( $rij ), '|' ) ) );
			$kop    = $cellen( $t );
			$i     += 2;
			$rijen  = array();
			while ( $i < $n && str_starts_with( ltrim( $regels[ $i ] ), '|' ) ) {
				$rijen[] = $cellen( $regels[ $i ] );
				++$i;
			}
			--$i;
			$th   = implode( '', array_map( static fn( $c ) => '<th>' . dg_inhoud_regel( $c, $link ) . '</th>', $kop ) );
			$body = '';
			foreach ( $rijen as $rij ) {
				$rij   = array_pad( array_slice( $rij, 0, count( $kop ) ), count( $kop ), '' );
				$body .= '<tr>' . implode( '', array_map( static fn( $c ) => '<td>' . dg_inhoud_regel( $c, $link ) . '</td>', $rij ) ) . '</tr>';
			}
			$blokken[] = dg_blok( 'table', '<figure class="wp-block-table"><table class="has-fixed-layout"><thead><tr>' . $th . '</tr></thead><tbody>' . $body . '</tbody></table></figure>' );
			continue;
		}
		$alinea[] = $t;
	}
	$sluit();
	return implode( "\n\n", $blokken );
}

/**
 * menu.md: per menu een kopje (## Hoofdmenu, ## Voet) met daaronder een lijst
 * links: - [Over het netwerk](over.md), - [Fora](/forum/), - [GitHub](https://…).
 *
 * @return array<string, list<array{0: string, 1: string}>>
 */
function dg_inhoud_menus( string $md ): array {
	$menus = array();
	$naam  = null;
	foreach ( explode( "\n", str_replace( "\r", '', $md ) ) as $regel ) {
		if ( preg_match( '/^##\s+(.+?)\s*$/', $regel, $m ) ) {
			$naam           = strtolower( preg_replace( '/[^a-z]/i', '', $m[1] ) );
			$menus[ $naam ] = array();
		} elseif ( null !== $naam && preg_match( '/^(\s*)[-*]\s+\[([^\]]+)\]\(([^)\s]+)\)\s*$/', $regel, $m ) ) {
			// Ingesprongen (twee spaties of meer): een onderdeel van het punt erboven.
			$menus[ $naam ][] = array( $m[2], $m[3], strlen( $m[1] ) >= 2 ? 1 : 0 );
		} elseif ( null !== $naam && preg_match( '/^[-*]\s+([^\[].*?)\s*$/', $regel, $m ) ) {
			// Een kopje zonder link, alleen om onderdelen onder te zetten.
			$menus[ $naam ][] = array( $m[1], '#', 0 );
		}
	}
	return $menus;
}

/**
 * Zet een menu op zijn plek in het thema: het hoofdmenu bovenaan (plek
 * "primary"). Andere menu's (zoals "voet") gebruikt het thema bij naam.
 */
function dg_inhoud_menuplek( string $naam, int $menu_id, bool $droog ): void {
	$plekken = array( 'hoofdmenu' => 'primary' );
	if ( $droog || ! isset( $plekken[ $naam ] ) ) {
		return;
	}
	$nu = get_theme_mod( 'nav_menu_locations', array() );
	if ( (int) ( $nu[ $plekken[ $naam ] ] ?? 0 ) !== $menu_id ) {
		$nu[ $plekken[ $naam ] ] = $menu_id;
		set_theme_mod( 'nav_menu_locations', $nu );
	}
}

// ---------------------------------------------------------------------------
// wp dg inhoud <map>
// ---------------------------------------------------------------------------

if ( defined( 'WP_CLI' ) && WP_CLI ) {
	/**
	 * Zet de pagina's uit <map>/paginas/ in WordPress.
	 *
	 * ## OPTIONS
	 *
	 * <map>
	 * : De map inhoud/ uit de repo.
	 *
	 * [--droog]
	 * : Alleen controleren en laten zien wat er zou veranderen.
	 */
	WP_CLI::add_command(
		'dg inhoud',
		static function ( $args, $opties ) {
			require_once ABSPATH . 'wp-admin/includes/file.php';
			require_once ABSPATH . 'wp-admin/includes/media.php';
			require_once ABSPATH . 'wp-admin/includes/image.php';
			$map   = rtrim( $args[0], '/' );
			$droog = ! empty( $opties['droog'] );
			if ( ! is_dir( "$map/paginas" ) ) {
				WP_CLI::error( "Geen map paginas/ in $map." );
			}

			// 1. Alles lezen en controleren, voordat er iets verandert.
			$paginas = array();
			$fouten  = array();
			$bestanden = glob( "$map/paginas/*.md" ) ?: array();
			sort( $bestanden );
			foreach ( $bestanden as $pad ) {
				$bron = 'paginas/' . basename( $pad );
				try {
					[ $kop, $tekst ] = dg_inhoud_lees( (string) file_get_contents( $pad ) );
				} catch ( RuntimeException $e ) {
					$fouten[] = "$bron: " . $e->getMessage();
					continue;
				}
				$adres  = $kop['adres'] ?? basename( $pad, '.md' );
				$status = $kop['status'] ?? 'publiceren';
				if ( ! preg_match( '/^[a-z0-9]+(-[a-z0-9]+)*$/', $adres ) ) {
					$fouten[] = "$bron: adres \"$adres\" mag alleen kleine letters, cijfers en streepjes bevatten.";
				}
				if ( empty( $kop['titel'] ) ) {
					$fouten[] = "$bron: titel ontbreekt in het kopje.";
				}
				if ( ! in_array( $status, array( 'publiceren', 'concept' ), true ) ) {
					$fouten[] = "$bron: status is \"publiceren\" of \"concept\", niet \"$status\".";
				}
				foreach ( dg_inhoud_controleer_vakken( $tekst ) as $fout ) {
					$fouten[] = "$bron: $fout";
				}
				foreach ( array_keys( $kop ) as $sleutel ) {
					if ( ! in_array( $sleutel, array( 'titel', 'adres', 'ouder', 'volgorde', 'status', 'voorpagina', 'dashboard' ), true ) ) {
						$fouten[] = "$bron: onbekend veld \"$sleutel\" in het kopje.";
					}
				}
				if ( isset( $paginas[ $adres ] ) ) {
					$fouten[] = "$bron: adres \"$adres\" komt al voor in " . $paginas[ $adres ]['bron'] . '.';
					continue;
				}
				$paginas[ $adres ] = array(
					'bron'       => $bron,
					'titel'      => $kop['titel'] ?? '',
					'ouder'      => $kop['ouder'] ?? '',
					'volgorde'   => (int) ( $kop['volgorde'] ?? 0 ),
					'status'     => 'concept' === $status ? 'draft' : 'publish',
					'dashboard'  => preg_replace( '/[^a-z]/', '', strtolower( $kop['dashboard'] ?? '' ) ),
					'voorpagina' => in_array( strtolower( $kop['voorpagina'] ?? '' ), array( 'ja', 'true', '1' ), true ),
					'tekst'      => $tekst,
				);
			}
			foreach ( $paginas as $adres => $p ) {
				if ( '' !== $p['ouder'] && ! isset( $paginas[ $p['ouder'] ] ) ) {
					$fouten[] = "{$p['bron']}: ouder \"{$p['ouder']}\" is geen pagina in de repo.";
				}
			}
			if ( count( array_filter( $paginas, static fn( $p ) => $p['voorpagina'] ) ) > 1 ) {
				$fouten[] = 'Meer dan één pagina heeft "voorpagina: ja".';
			}
			// Het volledige pad (ouder/kind) voor links tussen pagina's.
			$pad_van = static function ( string $adres ) use ( $paginas ): string {
				$delen = array();
				for ( $a = $adres, $diepte = 0; '' !== $a && $diepte < 10; $a = $paginas[ $a ]['ouder'] ?? '', $diepte++ ) {
					array_unshift( $delen, $a );
				}
				return implode( '/', $delen );
			};
			foreach ( $paginas as $adres => $p ) {
				if ( substr_count( $pad_van( $adres ), '/' ) >= 9 ) {
					$fouten[] = "{$p['bron']}: de ouders vormen een kring.";
				}
			}
			// Links naar andere pagina's (naam.md) en afbeeldingen (media/…) moeten bestaan.
			$per_bestand = array();
			foreach ( $paginas as $adres => $p ) {
				$per_bestand[ basename( $p['bron'] ) ] = $adres;
			}
			foreach ( $paginas as $p ) {
				preg_match_all( '/\]\(([^)\s]+)\)|^\s*(?:beeld|link):\s*(\S+)/m', $p['tekst'], $m );
				foreach ( array_filter( array_merge( $m[1], $m[2] ) ) as $doel ) {
					if ( preg_match( '#^[^:/]+\.md(\#.*)?$#', $doel ) && ! isset( $per_bestand[ strtok( $doel, '#' ) ] ) ) {
						$fouten[] = "{$p['bron']}: link naar $doel, maar die pagina bestaat niet.";
					}
					if ( str_starts_with( $doel, 'media/' ) && ( str_contains( $doel, '..' ) || ! is_file( "$map/$doel" ) ) ) {
						$fouten[] = "{$p['bron']}: afbeelding $doel staat niet in inhoud/media/.";
					}
				}
			}
			if ( is_file( "$map/menu.md" ) ) {
				foreach ( dg_inhoud_menus( (string) file_get_contents( "$map/menu.md" ) ) as $naam => $items ) {
					foreach ( $items as [ , $doel ] ) {
						if ( preg_match( '#^[^:/]+\.md$#', $doel ) && ! isset( $per_bestand[ $doel ] ) ) {
							$fouten[] = "menu.md ($naam): link naar $doel, maar die pagina bestaat niet.";
						}
					}
				}
			}
			if ( $fouten ) {
				foreach ( $fouten as $f ) {
					WP_CLI::warning( $f );
				}
				WP_CLI::error( count( $fouten ) . ' fout(en) in de pagina\'s; er is niets veranderd.' );
			}

			// 2. Afbeeldingen.
			$beeld = static function ( string $doel ) use ( $map, $droog ): array {
				$bestand = "$map/$doel";
				$hash    = hash_file( 'sha256', $bestand );
				$bestaand = get_posts( array( 'post_type' => 'attachment', 'post_status' => 'any', 'meta_key' => '_dg_bron', 'meta_value' => $doel, 'numberposts' => 1, 'fields' => 'ids' ) );
				if ( $bestaand && get_post_meta( $bestaand[0], '_dg_hash', true ) === $hash ) {
					return array( $bestaand[0], (string) wp_get_attachment_image_url( $bestaand[0], 'large' ) );
				}
				if ( $droog ) {
					WP_CLI::log( "  afbeelding $doel: " . ( $bestaand ? 'vervangen' : 'nieuw' ) );
					return array( 0, '' );
				}
				$tijdelijk = wp_tempnam( basename( $doel ) );
				copy( $bestand, $tijdelijk );
				$id = media_handle_sideload( array( 'name' => basename( $doel ), 'tmp_name' => $tijdelijk ), 0 );
				if ( is_wp_error( $id ) ) {
					WP_CLI::error( "Afbeelding $doel: " . $id->get_error_message() );
				}
				update_post_meta( $id, '_dg_bron', $doel );
				update_post_meta( $id, '_dg_hash', $hash );
				if ( $bestaand ) {
					delete_post_meta( $bestaand[0], '_dg_bron' );
					wp_delete_attachment( $bestaand[0], true );
				}
				WP_CLI::log( "  afbeelding $doel" );
				return array( $id, (string) wp_get_attachment_image_url( $id, 'large' ) );
			};
			$link = static function ( string $doel ) use ( $per_bestand, $pad_van ): string {
				if ( preg_match( '#^([^:/]+\.md)(\#.*)?$#', $doel, $m ) ) {
					return home_url( '/' . $pad_van( $per_bestand[ $m[1] ] ) . '/' ) . ( $m[2] ?? '' );
				}
				return $doel;
			};

			// 3. Pagina's, ouders eerst.
			uksort( $paginas, static fn( $a, $b ) => substr_count( $pad_van( $a ), '/' ) <=> substr_count( $pad_van( $b ), '/' ) ?: strcmp( $a, $b ) );
			$ids  = array();
			$tel  = array( 'nieuw' => 0, 'bijgewerkt' => 0, 'gelijk' => 0, 'ingetrokken' => 0 );
			$hier = array();
			foreach ( $paginas as $adres => $p ) {
				$hier[] = $p['bron'];
				$inhoud = dg_inhoud_blokken( $p['tekst'], $link, $beeld );
				$ouder  = '' === $p['ouder'] ? 0 : ( $ids[ $p['ouder'] ] ?? 0 );
				$hash   = hash( 'sha256', DG_INHOUD_VERSIE . "\n" . wp_json_encode( array( $p['titel'], $adres, $ouder, $p['volgorde'], $p['status'], $inhoud ) ) );
				$gevonden = get_posts( array( 'post_type' => 'page', 'post_status' => 'any', 'meta_key' => '_dg_bron', 'meta_value' => $p['bron'], 'numberposts' => 1, 'fields' => 'ids' ) );
				$id       = $gevonden[0] ?? 0;
				if ( ! $id ) {
					// Een pagina met dit adres die nog niet uit de repo kwam: overnemen.
					$oud = get_page_by_path( $adres );
					$id  = $oud && ! dg_inhoud_bron( $oud ) && ( $oud->post_parent === $ouder ) ? $oud->ID : 0;
				}
				if ( $id && get_post_meta( $id, '_dg_hash', true ) === $hash ) {
					$ids[ $adres ] = $id;
					++$tel['gelijk'];
					continue;
				}
				$soort = $id ? 'bijgewerkt' : 'nieuw';
				++$tel[ $soort ];
				WP_CLI::log( "  /{$pad_van( $adres )}/ ($soort)" );
				if ( $droog ) {
					$ids[ $adres ] = $id;
					continue;
				}
				$bericht = array(
					'post_type'    => 'page',
					'post_title'   => $p['titel'],
					'post_name'    => $adres,
					'post_parent'  => $ouder,
					'menu_order'   => $p['volgorde'],
					'post_status'  => $p['status'],
					'post_content' => $inhoud,
					'comment_status' => 'closed',
					'ping_status'  => 'closed',
				);
				if ( $id ) {
					$bericht['ID'] = $id;
				}
				$id = wp_insert_post( wp_slash( $bericht ), true );
				if ( is_wp_error( $id ) ) {
					WP_CLI::error( "{$p['bron']}: " . $id->get_error_message() );
				}
				update_post_meta( $id, '_dg_bron', $p['bron'] );
				update_post_meta( $id, '_dg_hash', $hash );
				$ids[ $adres ] = $id;
			}

			// 4. Pagina's die niet meer in de repo staan: concept, en niet meer op slot.
			$weg = get_posts( array( 'post_type' => 'page', 'post_status' => 'any', 'meta_key' => '_dg_bron', 'numberposts' => -1, 'fields' => 'ids' ) );
			foreach ( $weg as $id ) {
				$bron = dg_inhoud_bron( $id );
				if ( in_array( $bron, $hier, true ) ) {
					continue;
				}
				++$tel['ingetrokken'];
				WP_CLI::log( "  $bron staat niet meer in de repo: concept" );
				if ( ! $droog ) {
					wp_update_post( array( 'ID' => $id, 'post_status' => 'draft' ) );
					delete_post_meta( $id, '_dg_bron' );
					delete_post_meta( $id, '_dg_hash' );
				}
			}

			// 5. De menu's uit menu.md: alleen naar pagina's die gepubliceerd zijn.
			if ( is_file( "$map/menu.md" ) ) {
				$menus = dg_inhoud_menus( (string) file_get_contents( "$map/menu.md" ) );
				foreach ( $menus as $naam => $items ) {
					$links      = array();
					$ouder_weg  = false;
					foreach ( $items as [ $label, $doel, $niveau ] ) {
						if ( $niveau && $ouder_weg ) {
							continue;
						}
						if ( preg_match( '#^([^:/]+\.md)$#', $doel, $m ) ) {
							$adres = $per_bestand[ $m[1] ];
							if ( 'publish' !== $paginas[ $adres ]['status'] ) {
								$ouder_weg = ! $niveau;
								continue;
							}
							$url = home_url( '/' . $pad_van( $adres ) . '/' );
						} elseif ( str_starts_with( $doel, '/' ) ) {
							$url = home_url( $doel );
						} else {
							$url = $doel;
						}
						if ( ! $niveau ) {
							$ouder_weg = false;
						}
						$links[] = array( $label, $url, $niveau );
					}
					$hash = hash( 'sha256', (string) wp_json_encode( $links ) );
					$menu = wp_get_nav_menu_object( (int) get_option( "dg_menu_$naam" ) );
					if ( $menu && get_term_meta( $menu->term_id, '_dg_hash', true ) === $hash ) {
						dg_inhoud_menuplek( $naam, $menu->term_id, $droog );
						continue;
					}
					WP_CLI::log( "  menu $naam" );
					if ( $droog ) {
						continue;
					}
					$menu_id = $menu ? $menu->term_id : wp_create_nav_menu( ucfirst( $naam ) );
					if ( is_wp_error( $menu_id ) ) {
						WP_CLI::error( "menu $naam: " . $menu_id->get_error_message() );
					}
					foreach ( (array) wp_get_nav_menu_items( $menu_id, array( 'post_status' => 'any' ) ) as $oud ) {
						wp_delete_post( $oud->ID, true );
					}
					$ouder = 0;
					foreach ( $links as $volgorde => [ $label, $url, $niveau ] ) {
						$item = wp_update_nav_menu_item( $menu_id, 0, array( 'menu-item-title' => $label, 'menu-item-url' => $url, 'menu-item-type' => 'custom', 'menu-item-status' => 'publish', 'menu-item-position' => $volgorde + 1, 'menu-item-parent-id' => $niveau ? $ouder : 0 ) );
						if ( ! $niveau && ! is_wp_error( $item ) ) {
							$ouder = $item;
						}
					}
					update_term_meta( $menu_id, '_dg_hash', $hash );
					update_option( "dg_menu_$naam", $menu_id );
					dg_inhoud_menuplek( $naam, $menu_id, false );
				}
			}

			// 6. Gegevens voor het dashboard (dg-dashboard.php): de kaarten van een
			//    pagina met "dashboard: <naam>" in het kopje.
			foreach ( $paginas as $p ) {
				if ( '' === $p['dashboard'] || $droog ) {
					continue;
				}
				if ( 'publish' !== $p['status'] ) {
					// Een concept komt niet op het dashboard.
					delete_option( 'dg_dashboard_' . $p['dashboard'] );
					continue;
				}
				// Alleen de kaarten in een ::: kaarten-vak; andere ###-koppen
				// (bijvoorbeeld in ::: stappen) horen niet op het dashboard.
				$bron = preg_match_all( '/^:::\s*kaarten\s*$(.*?)^:::\s*$/ms', $p['tekst'], $vakken ) ? implode( "\n", $vakken[1] ) : $p['tekst'];
				$kaarten = array();
				foreach ( array_slice( preg_split( '/^###\s+/m', "\n" . $bron ), 1 ) as $deel ) {
					[ $kop, $rest ]     = array_pad( explode( "\n", $deel, 2 ), 2, '' );
					[ $kenmerk, $rest ] = dg_inhoud_kenmerken( preg_replace( '/^\s*:::.*$/m', '', $rest ) );
					$kaarten[]          = array(
						'naam'  => trim( $kop ),
						'label' => $kenmerk['label'][0] ?? '',
						'icoon' => $kenmerk['icoon'][0] ?? '',
						'tekst' => trim( (string) strtok( trim( $rest ), "\n" ) ),
					);
				}
				update_option( 'dg_dashboard_' . $p['dashboard'], $kaarten, false );
			}

			// 7. De voorpagina, en de privacyverklaring voor WordPress zelf.
			foreach ( $paginas as $adres => $p ) {
				if ( $p['voorpagina'] && ! $droog && (int) get_option( 'page_on_front' ) !== $ids[ $adres ] ) {
					update_option( 'show_on_front', 'page' );
					update_option( 'page_on_front', $ids[ $adres ] );
					WP_CLI::log( "  voorpagina: /$adres/" );
				}
			}
			if ( ! $droog && isset( $ids['privacy'] ) && (int) get_option( 'wp_page_for_privacy_policy' ) !== $ids['privacy'] ) {
				update_option( 'wp_page_for_privacy_policy', $ids['privacy'] );
			}
			$samen = "{$tel['nieuw']} nieuw, {$tel['bijgewerkt']} bijgewerkt, {$tel['gelijk']} gelijk, {$tel['ingetrokken']} ingetrokken";
			$droog ? WP_CLI::log( "Droog: $samen." ) : WP_CLI::success( "Pagina's: $samen." );
		}
	);
}
