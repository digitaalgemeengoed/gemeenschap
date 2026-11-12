<?php
/**
 * Plugin Name: Digitaal GemeenGoed — nieuws en berichten
 * Description: Berichten van leden en van de redactie, in de stijl van LinkedIn: een nieuwsfeed op /nieuws/, een eigen pagina per bericht, tags (ook voor projecten, werkgroepen en de coöperatie) met een feed per tag, reacties (leuk, gefeliciteerd, inspirerend, nuttig), opmerkingen, delen, verzenden, reposten, foto's, video's en linkvoorbeelden, en nieuwsflitsen op de homepage.
 *
 * Zichtbaarheid: per bericht "openbaar" (iedereen, ook om te delen op LinkedIn)
 * of "alleen leden" (de standaard). Gasten zien van een ledenbericht niets: niet
 * in de feed, niet bij een tag, niet in het zoeken of de RSS; de pagina vraagt om
 * in te loggen. Opmerkingen en wie reageerde zijn altijd alleen voor leden.
 * Let op: geüploade foto's en video's staan, zoals alles in de mediabibliotheek,
 * op een adres dat niet geheim is als je het kent.
 *
 * Nieuwsflits: per bericht aan te vinken (titel nodig). De homepage toont de
 * lopende flitsen (titel, kleine afbeelding, twee à drie regels) tot de einddatum,
 * standaard DG_BER_FLITS_DAGEN dagen. Gasten zien alleen openbare flitsen.
 *
 * Vaste tags (term-meta dg_soort): "cooperatie", "werkgroep-<slug>" voor elke
 * werkgroep (BuddyPress-groep) en "project-<id>" voor elk openbaar project in
 * Redmine. Dezelfde namen als de chatkamers en de ruimtes in Documenten. Elk uur
 * bijgewerkt. Vrije tags maakt een lid zelf, met #woord in de tekst of in het
 * veld Tags.
 *
 * Privacy: foto's worden opnieuw opgeslagen zonder EXIF (locatie, camera). Een
 * ingebedde video (YouTube, Vimeo) laadt pas na een klik; YouTube via
 * youtube-nocookie.com. Linkvoorbeelden haalt de server op; de afbeelding komt in
 * de eigen mediabibliotheek, zodat lezers niets van andere sites laden.
 *
 * @package DigitaalGemeenGoed
 */

defined( 'ABSPATH' ) || exit;

const DG_BER_TYPE        = 'dg_bericht';
const DG_BER_TAG         = 'dg_tag';
const DG_BER_DB_VERSIE   = 1;
const DG_BER_VERSIE      = 3; // ophogen als de adressen (rewrite) veranderen
const DG_BER_FLITS_DAGEN = 14;
const DG_BER_MAX_TEKST   = 3000;
const DG_BER_MAX_FOTOS   = 4;
const DG_BER_FOTO_MB     = 12;
const DG_BER_VIDEO_MB    = 60;
const DG_BER_PER_PAGINA  = 10;
const DG_BER_REACTIES    = array(
	'leuk'          => array( '👍', 'Leuk' ),
	'gefeliciteerd' => array( '🎉', 'Gefeliciteerd' ),
	'inspirerend'   => array( '💡', 'Inspirerend' ),
	'nuttig'        => array( '🙏', 'Nuttig' ),
);

// ---------------------------------------------------------------------------
// Registreren
// ---------------------------------------------------------------------------

function dg_ber_tabel(): string {
	global $wpdb;
	return $wpdb->prefix . 'dg_reacties';
}

function dg_ber_installeer(): void {
	global $wpdb;
	require_once ABSPATH . 'wp-admin/includes/upgrade.php';
	$tabel = dg_ber_tabel();
	dbDelta(
		"CREATE TABLE $tabel (
			id bigint(20) unsigned NOT NULL AUTO_INCREMENT,
			bericht bigint(20) unsigned NOT NULL,
			lid bigint(20) unsigned NOT NULL,
			soort varchar(20) NOT NULL,
			wanneer datetime NOT NULL,
			PRIMARY KEY  (id),
			UNIQUE KEY bericht_lid (bericht,lid),
			KEY lid (lid)
		) {$wpdb->get_charset_collate()};"
	);
	update_option( 'dg_ber_db', DG_BER_DB_VERSIE );
}

add_action(
	'init',
	static function () {
		// De tags eerst: hun adressen (/nieuws/tag/…) moeten vóór die van de berichten (/nieuws/…) komen.
		register_taxonomy(
			DG_BER_TAG,
			DG_BER_TYPE,
			array(
				'labels'            => array(
					'name'          => 'Tags',
					'singular_name' => 'Tag',
					'search_items'  => 'Tags zoeken',
					'all_items'     => 'Alle tags',
					'edit_item'     => 'Tag bewerken',
					'add_new_item'  => 'Nieuwe tag',
				),
				'public'            => true,
				'hierarchical'      => false,
				'show_in_rest'      => false,
				'show_admin_column' => true,
				'rewrite'           => array( 'slug' => 'nieuws/tag', 'with_front' => false ),
			)
		);
		register_post_type(
			DG_BER_TYPE,
			array(
				'labels'          => array(
					'name'          => 'Berichten',
					'singular_name' => 'Bericht',
					'add_new_item'  => 'Nieuw bericht',
					'edit_item'     => 'Bericht bewerken',
					'all_items'     => 'Alle berichten',
					'menu_name'     => 'Nieuws',
				),
				'public'          => true,
				'show_in_rest'    => false,
				'has_archive'     => 'nieuws',
				'rewrite'         => array( 'slug' => 'nieuws', 'with_front' => false ),
				'supports'        => array( 'title', 'editor', 'author', 'thumbnail', 'comments' ),
				'taxonomies'      => array( DG_BER_TAG ),
				'menu_icon'       => 'dashicons-megaphone',
				'menu_position'   => 6,
				'capability_type' => 'post',
			)
		);
		register_post_status(
			'dg_verborgen',
			array(
				'label'                     => 'Verborgen',
				'public'                    => false,
				'protected'                 => true,
				'show_in_admin_all_list'    => true,
				'show_in_admin_status_list' => true,
				/* translators: %s: aantal */
				'label_count'               => _n_noop( 'Verborgen <span class="count">(%s)</span>', 'Verborgen <span class="count">(%s)</span>' ),
			)
		);
		if ( wp_installing() || ! is_blog_installed() ) {
			return;
		}
		if ( (int) get_option( 'dg_ber_db' ) !== DG_BER_DB_VERSIE ) {
			dg_ber_installeer();
		}
		// Ook als de adressen er (weer) niet zijn, bijvoorbeeld na een wp rewrite flush zonder deze plugin.
		$regels = get_option( 'rewrite_rules' );
		if ( (int) get_option( 'dg_ber_versie' ) !== DG_BER_VERSIE || ( is_array( $regels ) && ! isset( $regels['nieuws/?$'] ) ) ) {
			update_option( 'dg_ber_versie', DG_BER_VERSIE );
			add_action( 'wp_loaded', static fn() => flush_rewrite_rules( false ) );
		}
		if ( ! wp_next_scheduled( 'dg_ber_vaste_tags' ) ) {
			wp_schedule_event( time() + 60, 'hourly', 'dg_ber_vaste_tags' );
		}
	},
	5
);

/** Kerngroep, redactie en beheer modereren: verbergen en verwijderen. */
function dg_ber_moderator( int $uid = 0 ): bool {
	$uid = $uid ?: get_current_user_id();
	if ( ! $uid ) {
		return false;
	}
	if ( user_can( $uid, 'edit_others_posts' ) ) {
		return true;
	}
	$groepen = (array) get_user_meta( $uid, 'dg_groepen', true );
	return (bool) array_intersect( $groepen, array( '/netwerk/kerngroep', '/site/redactie', '/site/beheer' ) );
}

function dg_ber_openbaar( int $id ): bool {
	return 'openbaar' === get_post_meta( $id, '_dg_zichtbaar', true );
}

function dg_ber_mag_lezen( WP_Post $p ): bool {
	return DG_BER_TYPE === $p->post_type && 'publish' === $p->post_status && ( is_user_logged_in() || dg_ber_openbaar( $p->ID ) );
}

// ---------------------------------------------------------------------------
// Alleen leden: wat gasten niet zien
// ---------------------------------------------------------------------------

// Elke lijst (feed, tag, zoeken, RSS, widgets): voor gasten alleen openbare berichten.
add_filter(
	'posts_where',
	static function ( $where, $q ) {
		if ( is_user_logged_in() || is_admin() ) {
			return $where;
		}
		// De pagina van één bericht: template_redirect vraagt dan om in te loggen.
		// (is_singular( type ) kan hier nog niet: het bericht is nog niet gevonden.)
		if ( $q->is_main_query() && $q->is_singular && ( DG_BER_TYPE === $q->get( 'post_type' ) || '' !== (string) $q->get( DG_BER_TYPE ) ) ) {
			return $where;
		}
		global $wpdb;
		return $where . $wpdb->prepare(
			" AND ( {$wpdb->posts}.post_type <> %s OR {$wpdb->posts}.ID IN ( SELECT post_id FROM {$wpdb->postmeta} WHERE meta_key = '_dg_zichtbaar' AND meta_value = 'openbaar' ) )",
			DG_BER_TYPE
		);
	},
	10,
	2
);

// Opmerkingen bij berichten: alleen voor leden (ook niet in de RSS van opmerkingen).
add_filter(
	'comments_clauses',
	static function ( $clauses ) {
		if ( is_user_logged_in() || is_admin() ) {
			return $clauses;
		}
		global $wpdb;
		$clauses['where'] .= $wpdb->prepare( " AND comment_post_ID NOT IN ( SELECT ID FROM {$wpdb->posts} WHERE post_type = %s )", DG_BER_TYPE );
		return $clauses;
	}
);

add_filter(
	'comment_feed_where',
	static function ( $where ) {
		if ( is_user_logged_in() ) {
			return $where;
		}
		global $wpdb;
		return $where . $wpdb->prepare( " AND comment_post_ID NOT IN ( SELECT ID FROM {$wpdb->posts} WHERE post_type = %s )", DG_BER_TYPE );
	}
);

add_filter(
	'wp_sitemaps_post_types',
	static function ( $soorten ) {
		unset( $soorten[ DG_BER_TYPE ] );
		return $soorten;
	}
);
add_filter(
	'wp_sitemaps_taxonomies',
	static function ( $soorten ) {
		unset( $soorten[ DG_BER_TAG ] );
		return $soorten;
	}
);

// Alleen leden plaatsen opmerkingen; die van leden staan er meteen.
add_action(
	'pre_comment_on_post',
	static function ( $id ) {
		if ( DG_BER_TYPE === get_post_type( $id ) && ! is_user_logged_in() ) {
			wp_die( 'Log in om te reageren.', 'Inloggen', array( 'response' => 403 ) );
		}
	}
);
add_filter(
	'pre_comment_approved',
	static function ( $goed, $data ) {
		if ( 'spam' !== $goed && 'trash' !== $goed && ! empty( $data['user_id'] ) && DG_BER_TYPE === get_post_type( (int) $data['comment_post_ID'] ) ) {
			return 1;
		}
		return $goed;
	},
	20,
	2
);
add_filter(
	'comment_post_redirect',
	static function ( $url, $c ) {
		return DG_BER_TYPE === get_post_type( (int) $c->comment_post_ID ) ? get_permalink( (int) $c->comment_post_ID ) . '#opmerking-' . (int) $c->comment_ID : $url;
	},
	10,
	2
);

// ---------------------------------------------------------------------------
// Tags
// ---------------------------------------------------------------------------

add_action( 'dg_ber_vaste_tags', 'dg_ber_vaste_tags' );

/** De vaste tags: coöperatie, werkgroepen (BuddyPress) en openbare projecten (Redmine). */
function dg_ber_vaste_tags(): void {
	$vast = array( 'cooperatie' => array( 'Coöperatie', 'cooperatie', '' ) );
	if ( function_exists( 'groups_get_groups' ) ) {
		$groepen = groups_get_groups( array( 'per_page' => 500, 'show_hidden' => false ) );
		foreach ( $groepen['groups'] as $g ) {
			$vast[ 'werkgroep-' . $g->slug ] = array( wp_specialchars_decode( (string) $g->name, ENT_QUOTES ), 'werkgroep', (string) $g->slug );
		}
	}
	if ( function_exists( 'dg_db_redmine' ) ) {
		$projecten = dg_db_redmine( '/projects.json?limit=100' );
		foreach ( (array) ( $projecten['projects'] ?? array() ) as $p ) {
			if ( ! empty( $p['identifier'] ) ) {
				$vast[ 'project-' . sanitize_title( (string) $p['identifier'] ) ] = array( (string) $p['name'], 'project', (string) $p['identifier'] );
			}
		}
	}
	foreach ( $vast as $slug => [ $naam, $soort, $koppeling ] ) {
		$term = get_term_by( 'slug', $slug, DG_BER_TAG );
		if ( ! $term ) {
			$nieuw = wp_insert_term( $naam, DG_BER_TAG, array( 'slug' => $slug ) );
			if ( is_wp_error( $nieuw ) ) {
				continue;
			}
			$id = (int) $nieuw['term_id'];
		} else {
			$id = (int) $term->term_id;
			if ( dg_ber_tagnaam( $term ) !== $naam ) {
				wp_update_term( $id, DG_BER_TAG, array( 'name' => $naam ) );
			}
		}
		update_term_meta( $id, 'dg_soort', $soort );
		update_term_meta( $id, 'dg_koppeling', $koppeling );
	}
	update_option( 'dg_ber_vaste_tags', time(), false );
}

/** De naam van een tag zoals hij er staat (WordPress bewaart & als &amp;). */
function dg_ber_tagnaam( WP_Term $t ): string {
	return wp_specialchars_decode( $t->name, ENT_QUOTES );
}

/** Een tag bij een woord: de slug zelf, of het project of de werkgroep met die naam. */
function dg_ber_zoek_tag( string $woord ): ?WP_Term {
	static $cache = array();
	$slug = sanitize_title( $woord );
	if ( '' === $slug ) {
		return null;
	}
	if ( ! array_key_exists( $slug, $cache ) ) {
		$cache[ $slug ] = null;
		foreach ( array( $slug, "project-$slug", "werkgroep-$slug" ) as $s ) {
			$t = get_term_by( 'slug', $s, DG_BER_TAG );
			if ( $t ) {
				$cache[ $slug ] = $t;
				break;
			}
		}
		if ( ! $cache[ $slug ] ) {
			$t              = get_term_by( 'name', trim( $woord ), DG_BER_TAG );
			$cache[ $slug ] = $t ?: null;
		}
	}
	return $cache[ $slug ];
}

/** De tags uit het veld Tags en de #woorden in de tekst; ontbrekende vrije tags komen erbij. */
function dg_ber_tags_uit( string $invoer, string $tekst ): array {
	$woorden = preg_split( '/[,;]+/u', $invoer );
	if ( preg_match_all( '/(?:^|[\s(])#([\p{L}\p{N}][\p{L}\p{N}_-]{0,39})/u', $tekst, $m ) ) {
		$woorden = array_merge( $woorden, $m[1] );
	}
	$ids = array();
	foreach ( $woorden as $w ) {
		$w = trim( ltrim( trim( (string) $w ), '#' ) );
		if ( '' === $w || mb_strlen( $w ) > 40 ) {
			continue;
		}
		$term = dg_ber_zoek_tag( $w );
		if ( ! $term ) {
			$slug = sanitize_title( $w );
			// Projecten, werkgroepen en de coöperatie zijn vaste tags; die maakt een lid niet zelf.
			if ( '' === $slug || preg_match( '/^(project|werkgroep)-/', $slug ) || 'cooperatie' === $slug ) {
				continue;
			}
			$nieuw = wp_insert_term( sanitize_text_field( $w ), DG_BER_TAG, array( 'slug' => $slug ) );
			if ( is_wp_error( $nieuw ) ) {
				continue;
			}
			$ids[] = (int) $nieuw['term_id'];
		} else {
			$ids[] = (int) $term->term_id;
		}
		if ( count( $ids ) >= 10 ) {
			break;
		}
	}
	return array_values( array_unique( $ids ) );
}

/** Wat bij een vaste tag hoort: de werkgroep of het project, de chatkamer en Documenten. */
function dg_ber_tag_links( WP_Term $t ): array {
	$soort = (string) get_term_meta( $t->term_id, 'dg_soort', true );
	$k     = (string) get_term_meta( $t->term_id, 'dg_koppeling', true );
	$links = array();
	if ( 'werkgroep' === $soort && function_exists( 'groups_get_id' ) ) {
		$g = groups_get_group( (int) groups_get_id( $k ) );
		if ( $g && $g->id ) {
			$links[] = array( 'mensen', 'Werkgroep', bp_get_group_url( $g ) );
		}
	}
	if ( 'project' === $soort ) {
		$p = get_page_by_path( $k );
		if ( $p ) {
			$links[] = array( 'lagen', 'Over het project', get_permalink( $p ) );
		}
		$redmine = rtrim( (string) getenv( 'DG_REDMINE_URL' ), '/' );
		if ( $redmine ) {
			$links[] = array( 'taken', 'Project in Redmine', $redmine . '/projects/' . rawurlencode( $k ) );
		}
	}
	if ( $soort && is_user_logged_in() ) {
		if ( function_exists( 'dg_chat_url' ) ) {
			$links[] = array( 'chat', 'Chat', dg_chat_url( $t->slug ) );
		}
		if ( 'cooperatie' !== $soort && function_exists( 'dg_documenten_url' ) ) {
			$links[] = array( 'document', 'Documenten', dg_documenten_url( 'r/' . ( 'project' === $soort ? $k : $t->slug ) ) );
		}
	}
	return $links;
}

function dg_ber_soortnaam( WP_Term $t ): string {
	return array(
		'werkgroep'  => 'Werkgroep',
		'project'    => 'Project',
		'cooperatie' => 'De coöperatie',
	)[ (string) get_term_meta( $t->term_id, 'dg_soort', true ) ] ?? 'Tag';
}

// ---------------------------------------------------------------------------
// Media en linkvoorbeelden
// ---------------------------------------------------------------------------

function dg_ber_media_includes(): void {
	require_once ABSPATH . 'wp-admin/includes/file.php';
	require_once ABSPATH . 'wp-admin/includes/media.php';
	require_once ABSPATH . 'wp-admin/includes/image.php';
}

/**
 * Een afbeelding in de mediabibliotheek, eerst opnieuw opgeslagen: rechtop gezet,
 * hooguit $max pixels, en zonder EXIF (locatie, camera). Geeft het nummer of een fout.
 */
function dg_ber_bewaar_beeld( string $pad, string $naam, int $post_id, int $max = 2560 ) {
	dg_ber_media_includes();
	$type = wp_check_filetype_and_ext( $pad, $naam );
	if ( ! in_array( $type['type'], array( 'image/jpeg', 'image/png', 'image/webp', 'image/gif' ), true ) ) {
		return new WP_Error( 'dg_soort', "$naam is geen foto (jpg, png, webp of gif)." );
	}
	$basis = sanitize_file_name( pathinfo( $naam, PATHINFO_FILENAME ) ) ?: 'foto';
	$doel  = wp_tempnam( $basis ) . '.' . $type['ext'];
	if ( ! copy( $pad, $doel ) ) {
		return new WP_Error( 'dg_kopie', 'De foto kon niet worden opgeslagen.' );
	}
	if ( 'image/gif' !== $type['type'] ) {
		$ed = wp_get_image_editor( $doel );
		if ( is_wp_error( $ed ) ) {
			wp_delete_file( $doel );
			return new WP_Error( 'dg_beeld', "$naam kon niet worden gelezen." );
		}
		if ( method_exists( $ed, 'maybe_exif_rotate' ) ) {
			$ed->maybe_exif_rotate();
		}
		$maat = $ed->get_size();
		if ( $maat['width'] > $max || $maat['height'] > $max ) {
			$ed->resize( $max, $max, false );
		}
		$ed->set_quality( 85 );
		$uit = $ed->save( $doel, $type['type'] );
		if ( is_wp_error( $uit ) ) {
			wp_delete_file( $doel );
			return $uit;
		}
		$doel = $uit['path'];
	}
	$id = media_handle_sideload( array( 'name' => $basis . '.' . $type['ext'], 'tmp_name' => $doel ), $post_id );
	if ( is_wp_error( $id ) ) {
		wp_delete_file( $doel );
	}
	return $id;
}

/** De geüploade foto's (fotos[]) en video; geeft de nummers en de foutmeldingen. */
function dg_ber_uploads( int $post_id, int $al ): array {
	dg_ber_media_includes();
	$ids    = array();
	$fouten = array();
	$f      = $_FILES['fotos'] ?? null; // phpcs:ignore WordPress.Security.ValidatedSanitizedInput, WordPress.Security.NonceVerification -- gecontroleerd in de handler.
	if ( is_array( $f ) && is_array( $f['name'] ?? null ) ) {
		foreach ( array_keys( $f['name'] ) as $i ) {
			$fout = (int) $f['error'][ $i ];
			if ( UPLOAD_ERR_NO_FILE === $fout ) {
				continue;
			}
			$naam = sanitize_file_name( (string) $f['name'][ $i ] );
			if ( $al + count( $ids ) >= DG_BER_MAX_FOTOS ) {
				$fouten[] = 'Hooguit ' . DG_BER_MAX_FOTOS . " foto's per bericht.";
				break;
			}
			if ( UPLOAD_ERR_OK !== $fout || (int) $f['size'][ $i ] > DG_BER_FOTO_MB * MB_IN_BYTES ) {
				$fouten[] = "$naam is te groot (hooguit " . DG_BER_FOTO_MB . ' MB).';
				continue;
			}
			$id = dg_ber_bewaar_beeld( (string) $f['tmp_name'][ $i ], $naam, $post_id );
			if ( is_wp_error( $id ) ) {
				$fouten[] = $id->get_error_message();
			} else {
				$ids[] = (int) $id;
			}
		}
	}
	$v = $_FILES['video'] ?? null; // phpcs:ignore WordPress.Security.ValidatedSanitizedInput, WordPress.Security.NonceVerification
	if ( is_array( $v ) && UPLOAD_ERR_NO_FILE !== (int) ( $v['error'] ?? UPLOAD_ERR_NO_FILE ) ) {
		$naam = sanitize_file_name( (string) $v['name'] );
		$type = wp_check_filetype_and_ext( (string) $v['tmp_name'], $naam );
		if ( UPLOAD_ERR_OK !== (int) $v['error'] || (int) $v['size'] > DG_BER_VIDEO_MB * MB_IN_BYTES ) {
			$fouten[] = "$naam is te groot (hooguit " . DG_BER_VIDEO_MB . ' MB). Zet een langere video op YouTube, Vimeo of PeerTube en plak de link.';
		} elseif ( ! in_array( $type['type'], array( 'video/mp4', 'video/webm' ), true ) ) {
			$fouten[] = "$naam is geen mp4- of webm-video.";
		} else {
			$id = media_handle_sideload( array( 'name' => $naam, 'tmp_name' => (string) $v['tmp_name'] ), $post_id );
			if ( is_wp_error( $id ) ) {
				$fouten[] = $id->get_error_message();
			} else {
				$ids[] = (int) $id;
			}
		}
	}
	return array( $ids, $fouten );
}

/** De eerste link in een tekst. */
function dg_ber_eerste_link( string $tekst ): string {
	return preg_match( '#\bhttps?://[^\s<>"\']+#i', $tekst, $m ) ? rtrim( $m[0], '.,;:!?)' ) : '';
}

/** Een afbeelding van een andere site in de eigen mediabibliotheek (0 als het niet lukt). */
function dg_ber_beeld_van_url( string $url, int $post_id ): int {
	dg_ber_media_includes();
	$tmp = download_url( $url, 8 );
	if ( is_wp_error( $tmp ) ) {
		return 0;
	}
	if ( filesize( $tmp ) > 8 * MB_IN_BYTES ) {
		wp_delete_file( $tmp );
		return 0;
	}
	$info = wp_get_image_mime( $tmp );
	$ext  = array( 'image/jpeg' => 'jpg', 'image/png' => 'png', 'image/webp' => 'webp', 'image/gif' => 'gif' )[ $info ] ?? '';
	if ( ! $ext ) {
		wp_delete_file( $tmp );
		return 0;
	}
	$id = dg_ber_bewaar_beeld( $tmp, 'voorbeeld.' . $ext, $post_id, 1200 );
	wp_delete_file( $tmp );
	return is_wp_error( $id ) ? 0 : (int) $id;
}

/**
 * Een voorbeeld van een link: titel, tekst, site en afbeelding; voor YouTube,
 * Vimeo en dergelijke de speler (oEmbed). Leeg als de link niet deugt.
 */
function dg_ber_voorbeeld( string $url, int $post_id ): array {
	$url  = esc_url_raw( $url, array( 'http', 'https' ) );
	$host = strtolower( (string) wp_parse_url( $url, PHP_URL_HOST ) );
	// Geen interne namen (redmine, keycloak): alleen echte domeinen.
	if ( '' === $host || ! str_contains( $host, '.' ) || ! wp_http_validate_url( $url ) ) {
		return array();
	}
	$eigen = strtolower( (string) wp_parse_url( home_url(), PHP_URL_HOST ) );
	if ( "documenten.$eigen" === $host ) {
		$naam = rawurldecode( basename( (string) wp_parse_url( $url, PHP_URL_PATH ) ) );
		return array( 'url' => $url, 'titel' => $naam ?: 'Documenten', 'tekst' => 'In Documenten, alleen voor leden.', 'site' => 'Documenten', 'soort' => 'documenten' );
	}
	if ( $eigen === $host ) {
		$pid = url_to_postid( $url );
		if ( $pid ) {
			$p = get_post( $pid );
			return array(
				'url'   => $url,
				'titel' => get_the_title( $p ),
				'tekst' => wp_trim_words( wp_strip_all_tags( $p->post_excerpt ?: strip_shortcodes( $p->post_content ) ), 28 ),
				'site'  => 'Digitaal GemeenGoed',
				'beeld' => (int) get_post_thumbnail_id( $p ),
				'eigen' => 1,
			);
		}
		return array( 'url' => $url, 'titel' => $url, 'site' => 'Digitaal GemeenGoed' );
	}
	// Een speler (YouTube, Vimeo, …) via oEmbed.
	$oe   = _wp_oembed_get_object();
	$prov = $oe->get_provider( $url, array( 'discover' => false ) );
	if ( $prov ) {
		$data = $oe->fetch( $prov, $url, array( 'width' => 720 ) );
		if ( $data && ! empty( $data->html ) && str_contains( (string) $data->html, '<iframe' ) ) {
			$html = (string) $oe->data2html( $data, $url );
			$html = str_replace( array( 'www.youtube.com/embed', '//youtube.com/embed' ), array( 'www.youtube-nocookie.com/embed', '//www.youtube-nocookie.com/embed' ), $html );
			$v    = array(
				'url'   => $url,
				'titel' => (string) ( $data->title ?? '' ),
				'site'  => (string) ( $data->provider_name ?? $host ),
				'embed' => $html,
			);
			if ( ! empty( $data->thumbnail_url ) ) {
				$v['beeld'] = dg_ber_beeld_van_url( (string) $data->thumbnail_url, $post_id );
			}
			return $v;
		}
	}
	// Een gewone pagina: de Open Graph-gegevens.
	$r = wp_safe_remote_get(
		$url,
		array(
			'timeout'             => 6,
			'redirection'         => 3,
			'limit_response_size' => 800000,
			'user-agent'          => 'Mozilla/5.0 (compatible; DigitaalGemeenGoed/1.0; +' . home_url( '/' ) . ')',
		)
	);
	$v = array( 'url' => $url, 'titel' => $host, 'site' => preg_replace( '/^www\./', '', $host ) );
	if ( is_wp_error( $r ) || 200 !== (int) wp_remote_retrieve_response_code( $r ) || ! str_contains( (string) wp_remote_retrieve_header( $r, 'content-type' ), 'html' ) ) {
		return $v;
	}
	$doc = new DOMDocument();
	libxml_use_internal_errors( true );
	$doc->loadHTML( '<?xml encoding="UTF-8">' . wp_remote_retrieve_body( $r ) );
	libxml_clear_errors();
	$meta = array();
	foreach ( $doc->getElementsByTagName( 'meta' ) as $m ) {
		$k = strtolower( $m->getAttribute( 'property' ) ?: $m->getAttribute( 'name' ) );
		if ( $k && ! isset( $meta[ $k ] ) ) {
			$meta[ $k ] = trim( $m->getAttribute( 'content' ) );
		}
	}
	$titel = $doc->getElementsByTagName( 'title' )->item( 0 );
	$v['titel'] = wp_strip_all_tags( $meta['og:title'] ?? ( $titel ? $titel->textContent : '' ) ) ?: $v['titel'];
	$v['tekst'] = wp_trim_words( wp_strip_all_tags( $meta['og:description'] ?? ( $meta['description'] ?? '' ) ), 30 );
	$v['site']  = wp_strip_all_tags( $meta['og:site_name'] ?? '' ) ?: $v['site'];
	$beeld      = $meta['og:image'] ?? ( $meta['twitter:image'] ?? '' );
	if ( $beeld ) {
		$v['beeld'] = dg_ber_beeld_van_url( WP_Http::make_absolute_url( $beeld, $url ), $post_id );
	}
	return array_map( static fn( $x ) => is_string( $x ) ? mb_substr( trim( $x ), 0, 400 ) : $x, $v );
}

// ---------------------------------------------------------------------------
// Opslaan, verwijderen, verbergen
// ---------------------------------------------------------------------------

/** Een melding voor de volgende pagina (zoals bij uitnodigen). */
function dg_ber_melding_zet( array $regels, bool $fout = false ): void {
	set_transient( 'dg_ber_melding_' . get_current_user_id(), array( $regels, $fout ), 10 * MINUTE_IN_SECONDS );
}

function dg_ber_melding(): string {
	if ( ! is_user_logged_in() ) {
		return '';
	}
	$m = get_transient( 'dg_ber_melding_' . get_current_user_id() );
	if ( ! is_array( $m ) ) {
		return '';
	}
	delete_transient( 'dg_ber_melding_' . get_current_user_id() );
	[ $regels, $fout ] = $m;
	return '<div class="dg-cf-melding' . ( $fout ? ' dg-cf-melding-fout' : '' ) . '" role="status"><ul>' . implode( '', array_map( static fn( $r ) => '<li>' . esc_html( $r ) . '</li>', (array) $regels ) ) . '</ul></div>';
}

add_action( 'admin_post_dg_bericht_opslaan', 'dg_ber_opslaan' );
add_action(
	'admin_post_nopriv_dg_bericht_opslaan',
	static function () {
		auth_redirect();
	}
);

function dg_ber_opslaan(): void {
	$uid   = get_current_user_id();
	$terug = wp_get_referer() ?: get_post_type_archive_link( DG_BER_TYPE );
	// Groter dan post_max_size: dan is $_POST leeg.
	if ( empty( $_POST ) && (int) ( $_SERVER['CONTENT_LENGTH'] ?? 0 ) > 0 ) {
		dg_ber_melding_zet( array( 'Wat je meestuurde was te groot. Een video mag hooguit ' . DG_BER_VIDEO_MB . ' MB zijn; plaats een langere video op YouTube, Vimeo of PeerTube en plak de link.' ), true );
		wp_safe_redirect( $terug );
		exit;
	}
	if ( ! $uid || ! check_admin_referer( 'dg_bericht_opslaan' ) ) {
		wp_die( 'Verlopen formulier. Ga terug, ververs de pagina en probeer het opnieuw.', 'Verlopen', array( 'response' => 403 ) );
	}
	$id     = absint( $_POST['id'] ?? 0 );
	$oud    = $id ? get_post( $id ) : null;
	$repost = absint( $_POST['repost_van'] ?? 0 );
	$orig   = $repost ? get_post( $repost ) : null;
	if ( $id && ( ! $oud || DG_BER_TYPE !== $oud->post_type || (int) $oud->post_author !== $uid ) ) {
		wp_die( 'Dit bericht kun je niet bewerken.', 'Niet toegestaan', array( 'response' => 403 ) );
	}
	if ( $repost && ( ! $orig || ! dg_ber_mag_lezen( $orig ) ) ) {
		dg_ber_melding_zet( array( 'Dat bericht bestaat niet meer.' ), true );
		wp_safe_redirect( $terug );
		exit;
	}
	// Een repost van een repost wijst naar het origineel.
	if ( $orig && get_post_meta( $orig->ID, '_dg_repost_van', true ) ) {
		$ouder = get_post( (int) get_post_meta( $orig->ID, '_dg_repost_van', true ) );
		$orig  = $ouder && dg_ber_mag_lezen( $ouder ) ? $ouder : $orig;
	}
	$tekst    = mb_substr( trim( sanitize_textarea_field( wp_unslash( $_POST['tekst'] ?? '' ) ) ), 0, DG_BER_MAX_TEKST );
	$titel    = mb_substr( trim( sanitize_text_field( wp_unslash( $_POST['titel'] ?? '' ) ) ), 0, 140 );
	$openbaar = 'openbaar' === sanitize_key( wp_unslash( $_POST['zichtbaar'] ?? '' ) );
	$flits    = ! $orig && ! empty( $_POST['flits'] );
	$link     = esc_url_raw( trim( (string) wp_unslash( $_POST['link'] ?? '' ) ), array( 'http', 'https' ) );
	$weg      = array_map( 'absint', (array) ( $_POST['weg'] ?? array() ) );
	// Een repost van een ledenbericht blijft voor leden.
	if ( $orig && ! dg_ber_openbaar( $orig->ID ) ) {
		$openbaar = false;
	}
	$fouten  = array();
	$heeft   = '' !== $tekst || $orig || ! empty( $_FILES['fotos']['name'][0] ) || ! empty( $_FILES['video']['name'] ) || ( $oud && get_post_meta( $oud->ID, '_dg_media', true ) );
	if ( ! $heeft && '' === $link ) {
		dg_ber_melding_zet( array( 'Schrijf iets, of voeg een foto, video of link toe.' ), true );
		wp_safe_redirect( $terug );
		exit;
	}
	if ( $flits && '' === $titel ) {
		dg_ber_melding_zet( array( 'Een nieuwsflits heeft een titel nodig. Vul een titel in, of zet het vinkje bij "nieuwsflits" uit.' ), true );
		wp_safe_redirect( $terug );
		exit;
	}
	$velden = array(
		'post_type'      => DG_BER_TYPE,
		'post_title'     => $titel,
		'post_content'   => $tekst,
		'post_author'    => $uid,
		'comment_status' => 'open',
		'ping_status'    => 'closed',
	);
	if ( '' === $titel ) {
		$velden['post_name'] = sanitize_title( wp_trim_words( $tekst ?: ( $orig ? $orig->post_title . ' ' . $orig->post_content : 'bericht' ), 8, '' ) ) ?: 'bericht';
	}
	if ( $oud ) {
		$velden['ID'] = $oud->ID;
		unset( $velden['post_author'] );
		$pid = wp_update_post( wp_slash( $velden ), true );
	} else {
		$velden['post_status'] = 'publish';
		$pid                   = wp_insert_post( wp_slash( $velden ), true );
	}
	if ( is_wp_error( $pid ) || ! $pid ) {
		dg_ber_melding_zet( array( 'Het bericht kon niet worden opgeslagen. Probeer het opnieuw.' ), true );
		wp_safe_redirect( $terug );
		exit;
	}
	$pid = (int) $pid;
	update_post_meta( $pid, '_dg_zichtbaar', $openbaar ? 'openbaar' : 'leden' );

	// Tags: uit het veld, de #woorden, en bij een repost die van het origineel.
	$tags = dg_ber_tags_uit( (string) wp_unslash( $_POST['tags'] ?? '' ), $tekst );
	if ( $orig ) {
		$tags = array_unique( array_merge( $tags, wp_get_object_terms( $orig->ID, DG_BER_TAG, array( 'fields' => 'ids' ) ) ) );
		update_post_meta( $pid, '_dg_repost_van', $orig->ID );
		dg_ber_tel_reposts( $orig->ID );
	}
	wp_set_object_terms( $pid, array_map( 'intval', $tags ), DG_BER_TAG );

	// Media: weg wat is aangevinkt, erbij wat is geüpload.
	$media = array_values( array_diff( array_map( 'intval', (array) get_post_meta( $pid, '_dg_media', true ) ), $weg ) );
	foreach ( $weg as $w ) {
		if ( (int) wp_get_post_parent_id( $w ) === $pid ) {
			wp_delete_attachment( $w, true );
		}
	}
	$fotos_al          = count( array_filter( $media, static fn( $m ) => wp_attachment_is_image( $m ) ) );
	[ $nieuw, $fouten ] = dg_ber_uploads( $pid, $fotos_al );
	$media             = array_merge( $media, $nieuw );
	update_post_meta( $pid, '_dg_media', $media );

	// De link: de opgegeven, anders de eerste in de tekst. Alleen opnieuw ophalen als hij veranderde.
	$link = $link ?: dg_ber_eerste_link( $tekst );
	$was  = (array) get_post_meta( $pid, '_dg_link', true );
	if ( $link && ( $was['url'] ?? '' ) !== $link ) {
		if ( ! empty( $was['beeld'] ) && empty( $was['eigen'] ) ) {
			wp_delete_attachment( (int) $was['beeld'], true );
		}
		update_post_meta( $pid, '_dg_link', dg_ber_voorbeeld( $link, $pid ) );
	} elseif ( ! $link && $was ) {
		if ( ! empty( $was['beeld'] ) && empty( $was['eigen'] ) ) {
			wp_delete_attachment( (int) $was['beeld'], true );
		}
		delete_post_meta( $pid, '_dg_link' );
	}

	// De uitgelichte afbeelding (voor de flits en het kaartje bij het delen): de eerste foto, anders die van de link.
	$foto = 0;
	foreach ( $media as $m ) {
		if ( wp_attachment_is_image( $m ) ) {
			$foto = $m;
			break;
		}
	}
	$lk   = (array) get_post_meta( $pid, '_dg_link', true );
	$foto = $foto ?: (int) ( $lk['beeld'] ?? 0 );
	$foto ? set_post_thumbnail( $pid, $foto ) : delete_post_thumbnail( $pid );

	// De flits.
	if ( $flits ) {
		$tot = strtotime( sanitize_text_field( wp_unslash( $_POST['flits_tot'] ?? '' ) ) . ' 23:59:59 ' . wp_timezone_string() );
		$tot = $tot && $tot > time() ? min( $tot, time() + 90 * DAY_IN_SECONDS ) : time() + DG_BER_FLITS_DAGEN * DAY_IN_SECONDS;
		update_post_meta( $pid, '_dg_flits_tot', $tot );
	} else {
		delete_post_meta( $pid, '_dg_flits_tot' );
	}

	if ( ! $oud ) {
		dg_ber_activiteit( get_post( $pid ), (bool) $orig );
	}
	$regels = $oud ? array( 'Bericht bijgewerkt.' ) : array( $orig ? 'Gedeeld in de feed.' : 'Je bericht staat erop.' );
	dg_ber_melding_zet( array_merge( $regels, $fouten ), (bool) $fouten );
	// Nieuw: terug naar de feed (of de tag) waar het werd geplaatst; bewerkt of gerepost: de pagina van het bericht.
	$feed = $oud || $orig ? '' : strtok( (string) $terug, '#' );
	wp_safe_redirect( $feed && ! str_contains( $feed, 'bewerken=' ) ? $feed . '#bericht-' . $pid : get_permalink( $pid ) );
	exit;
}

function dg_ber_tel_reposts( int $id ): void {
	$n = count(
		get_posts(
			array(
				'post_type'      => DG_BER_TYPE,
				'post_status'    => 'publish',
				'meta_key'       => '_dg_repost_van', // phpcs:ignore WordPress.DB.SlowDBQuery
				'meta_value'     => $id, // phpcs:ignore WordPress.DB.SlowDBQuery
				'fields'         => 'ids',
				'posts_per_page' => -1,
				'suppress_filters' => true,
			)
		)
	);
	update_post_meta( $id, '_dg_reposts', $n );
}

add_action(
	'admin_post_dg_bericht_verwijderen',
	static function () {
		$id = absint( $_POST['id'] ?? 0 );
		$p  = get_post( $id );
		if ( ! $p || DG_BER_TYPE !== $p->post_type || ! check_admin_referer( 'dg_bericht_verwijderen_' . $id ) ) {
			wp_die( 'Dat lukte niet.', 'Fout', array( 'response' => 403 ) );
		}
		$eigen = (int) $p->post_author === get_current_user_id();
		if ( ! $eigen && ! dg_ber_moderator() ) {
			wp_die( 'Dit bericht kun je niet verwijderen.', 'Niet toegestaan', array( 'response' => 403 ) );
		}
		wp_delete_post( $id, true );
		dg_ber_melding_zet( array( 'Het bericht is verwijderd.' ) );
		wp_safe_redirect( get_post_type_archive_link( DG_BER_TYPE ) );
		exit;
	}
);

add_action(
	'admin_post_dg_bericht_verbergen',
	static function () {
		$id = absint( $_POST['id'] ?? 0 );
		$p  = get_post( $id );
		if ( ! $p || DG_BER_TYPE !== $p->post_type || ! check_admin_referer( 'dg_bericht_verbergen_' . $id ) || ! dg_ber_moderator() ) {
			wp_die( 'Dat lukte niet.', 'Fout', array( 'response' => 403 ) );
		}
		wp_update_post( array( 'ID' => $id, 'post_status' => 'dg_verborgen' ) );
		$schrijver = get_userdata( (int) $p->post_author );
		if ( $schrijver && is_email( $schrijver->user_email ) ) {
			wp_mail(
				$schrijver->user_email,
				'Je bericht op Digitaal GemeenGoed is verborgen',
				"Hallo {$schrijver->display_name},\n\nDe kerngroep heeft je bericht \"" . ( $p->post_title ?: wp_trim_words( $p->post_content, 10 ) ) . "\" verborgen, omdat het niet past bij de afspraken van de community (bijvoorbeeld persoonsgegevens of vertrouwelijke informatie). Vragen? Antwoord op deze mail of stuur de kerngroep een bericht.\n\nDigitaal GemeenGoed\n"
			);
		}
		dg_ber_melding_zet( array( 'Het bericht is verborgen. De schrijver krijgt een mail. Terugzetten kan in het beheer (Nieuws).' ) );
		wp_safe_redirect( get_post_type_archive_link( DG_BER_TYPE ) );
		exit;
	}
);

// Wat bij een bericht hoort, gaat mee als het wordt verwijderd.
add_action(
	'before_delete_post',
	static function ( $id ) {
		if ( DG_BER_TYPE !== get_post_type( $id ) ) {
			return;
		}
		foreach ( get_attached_media( '', $id ) as $m ) {
			wp_delete_attachment( $m->ID, true );
		}
		global $wpdb;
		$wpdb->delete( dg_ber_tabel(), array( 'bericht' => $id ) ); // phpcs:ignore WordPress.DB.DirectDatabaseQuery
		if ( function_exists( 'bp_activity_delete' ) ) {
			bp_activity_delete( array( 'type' => 'dg_bericht', 'secondary_item_id' => $id ) );
		}
		$orig = (int) get_post_meta( $id, '_dg_repost_van', true );
		if ( $orig ) {
			add_action( 'deleted_post', static fn() => dg_ber_tel_reposts( $orig ) );
		}
	}
);

// ---------------------------------------------------------------------------
// BuddyPress: een regel in de activiteit (ook in de stroom van de werkgroep)
// ---------------------------------------------------------------------------

add_action(
	'bp_register_activity_actions',
	static function () {
		if ( function_exists( 'bp_activity_set_action' ) ) {
			bp_activity_set_action( 'activity', 'dg_bericht', 'Nieuwe berichten', false, 'Berichten', array( 'activity', 'member', 'group', 'member_groups' ) );
		}
	}
);

function dg_ber_activiteit( WP_Post $p, bool $repost ): void {
	if ( ! function_exists( 'bp_activity_add' ) ) {
		return;
	}
	$component = 'activity';
	$item      = 0;
	$verborgen = false;
	foreach ( wp_get_object_terms( $p->ID, DG_BER_TAG ) as $t ) {
		if ( 'werkgroep' === get_term_meta( $t->term_id, 'dg_soort', true ) && function_exists( 'groups_get_id' ) ) {
			$gid = (int) groups_get_id( (string) get_term_meta( $t->term_id, 'dg_koppeling', true ) );
			if ( $gid ) {
				$g         = groups_get_group( $gid );
				$component = 'groups';
				$item      = $gid;
				$verborgen = 'public' !== $g->status;
				break;
			}
		}
	}
	$naam = bp_core_get_userlink( (int) $p->post_author );
	bp_activity_add(
		array(
			'user_id'           => (int) $p->post_author,
			'component'         => $component,
			'type'              => 'dg_bericht',
			'action'            => $naam . ( $repost ? ' deelde een bericht' : ' plaatste een bericht' ),
			'content'           => esc_html( $p->post_title ? $p->post_title . ' — ' . wp_trim_words( $p->post_content, 30 ) : wp_trim_words( $p->post_content, 40 ) ),
			'primary_link'      => get_permalink( $p ),
			'item_id'           => $item,
			'secondary_item_id' => $p->ID,
			'hide_sitewide'     => $verborgen,
		)
	);
}

// ---------------------------------------------------------------------------
// REST: reageren, melden, verzenden, leden zoeken
// ---------------------------------------------------------------------------

add_action(
	'rest_api_init',
	static function () {
		$lid = static fn() => is_user_logged_in();
		$id  = array( 'id' => array( 'validate_callback' => static fn( $v ) => is_numeric( $v ) ) );
		register_rest_route(
			'dg/v1',
			'/berichten/(?P<id>\d+)/reactie',
			array(
				'methods'             => 'POST',
				'permission_callback' => $lid,
				'args'                => $id,
				'callback'            => 'dg_ber_rest_reactie',
			)
		);
		register_rest_route(
			'dg/v1',
			'/berichten/(?P<id>\d+)/melden',
			array(
				'methods'             => 'POST',
				'permission_callback' => $lid,
				'args'                => $id,
				'callback'            => 'dg_ber_rest_melden',
			)
		);
		register_rest_route(
			'dg/v1',
			'/berichten/(?P<id>\d+)/sturen',
			array(
				'methods'             => 'POST',
				'permission_callback' => $lid,
				'args'                => $id,
				'callback'            => 'dg_ber_rest_sturen',
			)
		);
		register_rest_route(
			'dg/v1',
			'/leden',
			array(
				'methods'             => 'GET',
				'permission_callback' => $lid,
				'callback'            => 'dg_ber_rest_leden',
			)
		);
	}
);

function dg_ber_rest_bericht( WP_REST_Request $v ) {
	$p = get_post( (int) $v['id'] );
	return $p && dg_ber_mag_lezen( $p ) ? $p : new WP_Error( 'dg_niet_gevonden', 'Dit bericht bestaat niet (meer).', array( 'status' => 404 ) );
}

function dg_ber_rest_reactie( WP_REST_Request $v ) {
	$p = dg_ber_rest_bericht( $v );
	if ( is_wp_error( $p ) ) {
		return $p;
	}
	$soort = sanitize_key( (string) $v['soort'] );
	if ( '' !== $soort && ! isset( DG_BER_REACTIES[ $soort ] ) ) {
		return new WP_Error( 'dg_soort', 'Onbekende reactie.', array( 'status' => 400 ) );
	}
	global $wpdb;
	$tabel = dg_ber_tabel();
	$uid   = get_current_user_id();
	$mijn  = (string) $wpdb->get_var( $wpdb->prepare( "SELECT soort FROM $tabel WHERE bericht = %d AND lid = %d", $p->ID, $uid ) ); // phpcs:ignore WordPress.DB.PreparedSQL.InterpolatedNotPrepared, WordPress.DB.DirectDatabaseQuery
	$wpdb->delete( $tabel, array( 'bericht' => $p->ID, 'lid' => $uid ) ); // phpcs:ignore WordPress.DB.DirectDatabaseQuery
	// Dezelfde reactie nog eens: weer weg.
	if ( '' !== $soort && $soort !== $mijn ) {
		$wpdb->insert( $tabel, array( 'bericht' => $p->ID, 'lid' => $uid, 'soort' => $soort, 'wanneer' => current_time( 'mysql', true ) ) ); // phpcs:ignore WordPress.DB.DirectDatabaseQuery
	}
	dg_ber_reacties( array(), true );
	return array( 'telling' => dg_ber_telling_html( $p ), 'knop' => dg_ber_reageerknop_html( $p ) );
}

function dg_ber_rest_melden( WP_REST_Request $v ) {
	$p = dg_ber_rest_bericht( $v );
	if ( is_wp_error( $p ) ) {
		return $p;
	}
	$uid      = get_current_user_id();
	$meldingen = (array) get_post_meta( $p->ID, '_dg_meldingen', true );
	if ( isset( $meldingen[ $uid ] ) ) {
		return array( 'tekst' => 'Je had dit bericht al gemeld. De kerngroep kijkt ernaar.' );
	}
	$reden              = mb_substr( sanitize_textarea_field( (string) $v['reden'] ), 0, 1000 );
	$meldingen[ $uid ]  = array( time(), $reden );
	update_post_meta( $p->ID, '_dg_meldingen', $meldingen );
	$aan = array();
	foreach ( get_users( array( 'meta_key' => 'dg_groepen', 'meta_value' => '"/netwerk/kerngroep"', 'meta_compare' => 'LIKE', 'number' => 50 ) ) as $u ) { // phpcs:ignore WordPress.DB.SlowDBQuery
		$aan[] = $u->user_email;
	}
	$aan = $aan ?: array( get_option( 'admin_email' ) );
	$wie = wp_get_current_user();
	wp_mail(
		$aan,
		'Melding over een bericht op Digitaal GemeenGoed',
		"{$wie->display_name} meldt dit bericht:\n\n" . get_permalink( $p ) . "\n\nReden: " . ( $reden ?: '(geen reden gegeven)' ) . "\n\nVerbergen kan via het menu ⋯ bij het bericht.\n"
	);
	return array( 'tekst' => 'Bedankt. De kerngroep kijkt ernaar.' );
}

function dg_ber_rest_sturen( WP_REST_Request $v ) {
	$p = dg_ber_rest_bericht( $v );
	if ( is_wp_error( $p ) ) {
		return $p;
	}
	if ( ! function_exists( 'messages_new_message' ) ) {
		return new WP_Error( 'dg_geen_berichten', 'Privéberichten staan uit.', array( 'status' => 503 ) );
	}
	$aan   = array_slice( array_unique( array_filter( array_map( 'absint', (array) $v['aan'] ) ) ), 0, 10 );
	$tekst = mb_substr( trim( sanitize_textarea_field( (string) $v['tekst'] ) ), 0, 1000 );
	$uid   = get_current_user_id();
	$titel = $p->post_title ?: wp_trim_words( $p->post_content, 10 );
	$n     = 0;
	foreach ( $aan as $id ) {
		if ( $id === $uid || ! get_userdata( $id ) ) {
			continue;
		}
		$draad = messages_new_message(
			array(
				'sender_id'  => $uid,
				'recipients' => array( $id ),
				'subject'    => 'Bericht: ' . $titel,
				'content'    => ( '' !== $tekst ? esc_html( $tekst ) . "\n\n" : '' ) . '<a href="' . esc_url( get_permalink( $p ) ) . '">' . esc_html( $titel ) . '</a>',
				'error_type' => 'wp_error',
			)
		);
		if ( $draad && ! is_wp_error( $draad ) ) {
			++$n;
		}
	}
	return array( 'verstuurd' => $n, 'tekst' => $n ? ( 1 === $n ? 'Verstuurd.' : "Verstuurd naar $n leden." ) : 'Niet verstuurd: kies een lid.' );
}

function dg_ber_rest_leden( WP_REST_Request $v ) {
	$zoek = sanitize_text_field( (string) $v['zoek'] );
	if ( mb_strlen( $zoek ) < 2 ) {
		return array();
	}
	$uit = array();
	foreach (
		get_users(
			array(
				'search'         => '*' . $zoek . '*',
				'search_columns' => array( 'display_name', 'user_nicename' ),
				'number'         => 8,
				'exclude'        => array( get_current_user_id() ),
			)
		) as $u
	) {
		// De organisatie erbij; zonder organisatie de gebruikersnaam (twee keer "Jan de Vries" moet te onderscheiden zijn).
		$org   = (string) get_user_meta( $u->ID, 'dg_organisatie', true );
		$uit[] = array( 'id' => $u->ID, 'naam' => $u->display_name . ' (' . ( $org ?: '@' . $u->user_nicename ) . ')' );
	}
	return $uit;
}

// ---------------------------------------------------------------------------
// Weergave: een bericht als kaart
// ---------------------------------------------------------------------------

/** Alle reacties op de berichten in één keer: tellingen, de mijne, de namen. */
function dg_ber_reacties( array $ids, bool $leeg = false ): array {
	static $cache = array();
	if ( $leeg ) {
		$cache = array();
		return array();
	}
	$nodig = array_diff( array_map( 'intval', $ids ), array_keys( $cache ) );
	if ( $nodig ) {
		global $wpdb;
		$tabel = dg_ber_tabel();
		$lijst = implode( ',', $nodig );
		$rijen = (array) $wpdb->get_results( "SELECT bericht, lid, soort FROM $tabel WHERE bericht IN ($lijst) ORDER BY wanneer DESC", ARRAY_A ); // phpcs:ignore WordPress.DB.PreparedSQL.InterpolatedNotPrepared, WordPress.DB.DirectDatabaseQuery -- alleen getallen.
		foreach ( $nodig as $id ) {
			$cache[ $id ] = array( 'tel' => array(), 'totaal' => 0, 'mijn' => '', 'namen' => array() );
		}
		$uid = get_current_user_id();
		foreach ( $rijen as $r ) {
			$c = &$cache[ (int) $r['bericht'] ];
			$c['tel'][ $r['soort'] ] = ( $c['tel'][ $r['soort'] ] ?? 0 ) + 1;
			++$c['totaal'];
			if ( $uid && (int) $r['lid'] === $uid ) {
				$c['mijn'] = $r['soort'];
			}
			if ( count( $c['namen'] ) < 12 ) {
				$u            = get_userdata( (int) $r['lid'] );
				$c['namen'][] = $u ? $u->display_name : '';
			}
			unset( $c );
		}
	}
	return array_intersect_key( $cache, array_flip( array_map( 'intval', $ids ) ) );
}

function dg_ber_r( int $id ): array {
	return dg_ber_reacties( array( $id ) )[ $id ] ?? array( 'tel' => array(), 'totaal' => 0, 'mijn' => '', 'namen' => array() );
}

function dg_ber_tekst_html( string $tekst ): string {
	$h = make_clickable( esc_html( $tekst ) );
	$h = preg_replace_callback(
		'/(^|[\s(])#([\p{L}\p{N}][\p{L}\p{N}_-]{0,39})/u',
		static function ( $m ) {
			$t = dg_ber_zoek_tag( $m[2] );
			return $t ? $m[1] . '<a class="dg-ber-hashtag" href="' . esc_url( get_term_link( $t ) ) . '">#' . esc_html( $m[2] ) . '</a>' : $m[0];
		},
		$h
	);
	return wpautop( str_replace( '<a href=', '<a target="_blank" href=', $h ) );
}

function dg_ber_lid_html( int $uid, int $tijd, string $extra = '' ): string {
	$u       = get_userdata( $uid );
	$naam    = $u ? $u->display_name : 'Oud-lid';
	$org     = $u ? (string) get_user_meta( $uid, 'dg_organisatie', true ) : '';
	$profiel = $u && function_exists( 'bp_members_get_user_url' ) && is_user_logged_in() ? bp_members_get_user_url( $uid ) : '';
	$foto    = function_exists( 'bp_core_fetch_avatar' ) ? bp_core_fetch_avatar( array( 'item_id' => $uid, 'type' => 'thumb', 'width' => 48, 'height' => 48, 'html' => true, 'alt' => '' ) ) : get_avatar( $uid, 48 );
	$naam_h  = $profiel ? '<a href="' . esc_url( $profiel ) . '">' . esc_html( $naam ) . '</a>' : esc_html( $naam );
	return '<div class="dg-ber-wie">' . $foto . '<div><strong>' . $naam_h . '</strong>' . ( $org ? '<span>' . esc_html( $org ) . '</span>' : '' ) . '<small>' . $extra . '</small></div></div>';
}

function dg_ber_zichtbaar_html( int $id ): string {
	return dg_ber_openbaar( $id )
		? '<span class="dg-ber-zicht" title="Openbaar: iedereen kan dit lezen"><span class="dg-icoon dg-icoon-wereld" aria-hidden="true"></span>Openbaar</span>'
		: '<span class="dg-ber-zicht" title="Alleen leden kunnen dit lezen"><span class="dg-icoon dg-icoon-slot" aria-hidden="true"></span>Leden</span>';
}

function dg_ber_flits_actief( int $id ): bool {
	return (int) get_post_meta( $id, '_dg_flits_tot', true ) > time();
}

function dg_ber_media_html( WP_Post $p ): string {
	$media = array_filter( array_map( 'intval', (array) get_post_meta( $p->ID, '_dg_media', true ) ), static fn( $m ) => get_post( $m ) );
	if ( ! $media ) {
		return '';
	}
	$fotos = array_values( array_filter( $media, 'wp_attachment_is_image' ) );
	$h     = '';
	if ( $fotos ) {
		$h .= '<div class="dg-ber-fotos dg-ber-fotos-' . count( $fotos ) . '">';
		foreach ( $fotos as $f ) {
			$h .= '<a href="' . esc_url( (string) wp_get_attachment_url( $f ) ) . '" target="_blank">' . wp_get_attachment_image( $f, 'large', false, array( 'loading' => 'lazy', 'alt' => (string) get_post_meta( $f, '_wp_attachment_image_alt', true ) ) ) . '</a>';
		}
		$h .= '</div>';
	}
	foreach ( array_diff( $media, $fotos ) as $v ) {
		if ( str_starts_with( (string) get_post_mime_type( $v ), 'video/' ) ) {
			$h .= '<video class="dg-ber-video" controls preload="metadata" src="' . esc_url( (string) wp_get_attachment_url( $v ) ) . '"></video>';
		}
	}
	return $h;
}

function dg_ber_link_html( WP_Post $p ): string {
	$l = (array) get_post_meta( $p->ID, '_dg_link', true );
	if ( empty( $l['url'] ) ) {
		return '';
	}
	$beeld = ! empty( $l['beeld'] ) ? wp_get_attachment_image( (int) $l['beeld'], 'medium_large', false, array( 'loading' => 'lazy', 'alt' => '' ) ) : '';
	if ( ! empty( $l['embed'] ) ) {
		$site = esc_html( (string) ( $l['site'] ?? '' ) );
		return '<div class="dg-ber-embed"><button type="button" class="dg-ber-speel">' . $beeld
			. '<span class="dg-ber-speel-knop"><span aria-hidden="true">▶</span> Afspelen</span>'
			. '<small>' . esc_html( (string) ( $l['titel'] ?? '' ) ) . ' · laadt ' . $site . ' (dan gaan er gegevens naar ' . $site . ')</small></button>'
			. '<template>' . $l['embed'] . '</template></div>'; // phpcs:ignore WordPress.Security.EscapeOutput -- oEmbed van een vertrouwde aanbieder (WordPress-lijst).
	}
	$extern = empty( $l['eigen'] ) && 'documenten' !== ( $l['soort'] ?? '' );
	return '<a class="dg-ber-linkkaart' . ( $beeld ? ' met-beeld' : '' ) . '" href="' . esc_url( $l['url'] ) . '"' . ( $extern ? ' target="_blank" rel="noopener nofollow"' : '' ) . '>'
		. ( $beeld ?: '<span class="dg-ber-linkicoon"><span class="dg-icoon dg-icoon-' . ( 'documenten' === ( $l['soort'] ?? '' ) ? 'document' : 'link' ) . '" aria-hidden="true"></span></span>' )
		. '<span><small>' . esc_html( (string) ( $l['site'] ?? '' ) ) . '</small><strong>' . esc_html( (string) ( $l['titel'] ?? '' ) ) . '</strong>'
		. ( ! empty( $l['tekst'] ) ? '<span>' . esc_html( (string) $l['tekst'] ) . '</span>' : '' ) . '</span></a>';
}

function dg_ber_tags_html( WP_Post $p ): string {
	$tags = wp_get_object_terms( $p->ID, DG_BER_TAG );
	if ( ! $tags || is_wp_error( $tags ) ) {
		return '';
	}
	usort( $tags, static fn( $a, $b ) => (int) ! get_term_meta( $a->term_id, 'dg_soort', true ) <=> (int) ! get_term_meta( $b->term_id, 'dg_soort', true ) );
	return '<ul class="dg-ber-tags">' . implode(
		'',
		array_map(
			static fn( $t ) => '<li><a class="dg-ber-tag' . ( get_term_meta( $t->term_id, 'dg_soort', true ) ? ' is-vast' : '' ) . '" href="' . esc_url( get_term_link( $t ) ) . '">#' . esc_html( dg_ber_tagnaam( $t ) ) . '</a></li>',
			$tags
		)
	) . '</ul>';
}

function dg_ber_telling_html( WP_Post $p ): string {
	$r     = dg_ber_r( $p->ID );
	$opm   = (int) get_comments_number( $p );
	$rep   = (int) get_post_meta( $p->ID, '_dg_reposts', true );
	$delen = array();
	if ( $r['totaal'] ) {
		arsort( $r['tel'] );
		$emoji = implode( '', array_map( static fn( $s ) => '<span>' . DG_BER_REACTIES[ $s ][0] . '</span>', array_slice( array_keys( $r['tel'] ), 0, 3 ) ) );
		$wie   = is_user_logged_in() ? implode( ', ', array_filter( $r['namen'] ) ) . ( $r['totaal'] > count( $r['namen'] ) ? ' en anderen' : '' ) : '';
		$uitleg = implode( ', ', array_map( static fn( $s, $n ) => $n . ' ' . strtolower( DG_BER_REACTIES[ $s ][1] ), array_keys( $r['tel'] ), $r['tel'] ) );
		$delen[] = '<span class="dg-ber-emoji" title="' . esc_attr( trim( $uitleg . ( $wie ? ": $wie" : '' ) ) ) . '">' . $emoji . ' ' . (int) $r['totaal'] . '</span>';
	}
	if ( $opm ) {
		$delen[] = '<a href="' . esc_url( get_permalink( $p ) ) . '#opmerkingen">' . $opm . ( 1 === $opm ? ' opmerking' : ' opmerkingen' ) . '</a>';
	}
	if ( $rep ) {
		$delen[] = '<span>' . $rep . ' keer gedeeld</span>';
	}
	return '<div class="dg-ber-telling" data-telling>' . implode( '<span aria-hidden="true">·</span>', $delen ) . '</div>';
}

function dg_ber_reageerknop_html( WP_Post $p ): string {
	$mijn  = dg_ber_r( $p->ID )['mijn'];
	$hoofd = $mijn ?: 'leuk';
	$h     = '<div class="dg-ber-reageer" data-reageer><button type="button" class="dg-ber-actie' . ( $mijn ? ' is-gekozen' : '' ) . '" data-reactie="' . esc_attr( $hoofd ) . '" aria-pressed="' . ( $mijn ? 'true' : 'false' ) . '">'
		. '<span class="dg-ber-e" aria-hidden="true">' . DG_BER_REACTIES[ $hoofd ][0] . '</span>' . esc_html( DG_BER_REACTIES[ $hoofd ][1] ) . '</button>'
		. '<button type="button" class="dg-ber-meer-reacties" aria-label="Andere reactie kiezen" aria-expanded="false">▾</button>'
		. '<div class="dg-ber-keuze" role="group" aria-label="Reageer met">';
	foreach ( DG_BER_REACTIES as $s => [ $e, $label ] ) {
		$h .= '<button type="button" data-reactie="' . esc_attr( $s ) . '" title="' . esc_attr( $label ) . '"' . ( $s === $mijn ? ' class="is-gekozen"' : '' ) . '><span aria-hidden="true">' . $e . '</span><small>' . esc_html( $label ) . '</small></button>';
	}
	return $h . '</div></div>';
}

/** Delen met andere sites: alleen bij openbare berichten; anders alleen de link en e-mail. */
function dg_ber_delen_html( WP_Post $p ): string {
	$url    = get_permalink( $p );
	$titel  = $p->post_title ?: wp_trim_words( $p->post_content, 12 );
	$u      = rawurlencode( $url );
	$t      = rawurlencode( $titel );
	$links  = array( array( 'mail', 'E-mail', 'mailto:?subject=' . $t . '&body=' . rawurlencode( $titel . "\n\n" . $url ) ) );
	if ( dg_ber_openbaar( $p->ID ) ) {
		$links[] = array( 'delen', 'LinkedIn', 'https://www.linkedin.com/sharing/share-offsite/?url=' . $u );
		$links[] = array( 'delen', 'Bluesky', 'https://bsky.app/intent/compose?text=' . rawurlencode( $titel . ' ' . $url ) );
		$links[] = array( 'delen', 'Mastodon', 'https://share.joinmastodon.org/#text=' . rawurlencode( $titel . ' ' . $url ) );
		$links[] = array( 'delen', 'WhatsApp', 'https://wa.me/?text=' . rawurlencode( $titel . ' ' . $url ) );
		$links[] = array( 'delen', 'X', 'https://x.com/intent/post?url=' . $u . '&text=' . $t );
	}
	$h = '<details class="dg-ber-menu dg-ber-delen"><summary class="dg-ber-actie"><span class="dg-icoon dg-icoon-delen" aria-hidden="true"></span>Delen</summary><div class="dg-ber-uitklap">'
		. '<button type="button" data-kopieer="' . esc_attr( $url ) . '"><span class="dg-icoon dg-icoon-link" aria-hidden="true"></span>Link kopiëren</button>';
	foreach ( $links as [ $icoon, $naam, $href ] ) {
		$h .= '<a href="' . esc_url( $href, array( 'https', 'mailto' ) ) . '" target="_blank" rel="noopener"><span class="dg-icoon dg-icoon-' . $icoon . '" aria-hidden="true"></span>' . esc_html( $naam ) . '</a>';
	}
	if ( ! dg_ber_openbaar( $p->ID ) ) {
		$h .= '<p>Dit bericht is alleen voor leden: wie geen lid is, kan het niet lezen.</p>';
	}
	return $h . '</div></details>';
}

/** Het menu ⋯: bewerken, verwijderen, melden, verbergen. */
function dg_ber_beheer_html( WP_Post $p ): string {
	if ( ! is_user_logged_in() ) {
		return '';
	}
	$eigen = (int) $p->post_author === get_current_user_id();
	$mod   = dg_ber_moderator();
	$h     = '';
	if ( $eigen ) {
		$h .= '<a href="' . esc_url( add_query_arg( 'bewerken', 1, get_permalink( $p ) ) ) . '">Bewerken</a>';
	}
	if ( $eigen || $mod ) {
		$h .= '<form method="post" action="' . esc_url( admin_url( 'admin-post.php' ) ) . '" data-bevestig="Dit bericht verwijderen? Dat kan niet ongedaan worden gemaakt.">'
			. wp_nonce_field( 'dg_bericht_verwijderen_' . $p->ID, '_wpnonce', false, false )
			. '<input type="hidden" name="action" value="dg_bericht_verwijderen"><input type="hidden" name="id" value="' . (int) $p->ID . '"><button type="submit">Verwijderen</button></form>';
	}
	if ( ! $eigen ) {
		$h .= '<button type="button" data-melden="' . (int) $p->ID . '">Melden</button>';
	}
	if ( $mod && ! $eigen ) {
		$h .= '<form method="post" action="' . esc_url( admin_url( 'admin-post.php' ) ) . '" data-bevestig="Dit bericht verbergen? De schrijver krijgt een mail.">'
			. wp_nonce_field( 'dg_bericht_verbergen_' . $p->ID, '_wpnonce', false, false )
			. '<input type="hidden" name="action" value="dg_bericht_verbergen"><input type="hidden" name="id" value="' . (int) $p->ID . '"><button type="submit">Verbergen (kerngroep)</button></form>';
	}
	return '<details class="dg-ber-menu dg-ber-beheer"><summary aria-label="Meer"><span aria-hidden="true">⋯</span></summary><div class="dg-ber-uitklap">' . $h . '</div></details>';
}

/** Een bericht als kaart; $volledig op de eigen pagina (geen "meer lezen"). */
function dg_ber_kaart( WP_Post $p, bool $volledig = false ): string {
	$tijd   = (int) get_post_time( 'U', true, $p );
	$link   = get_permalink( $p );
	$orig   = (int) get_post_meta( $p->ID, '_dg_repost_van', true );
	$extra  = '<a href="' . esc_url( $link ) . '"><time datetime="' . esc_attr( gmdate( 'c', $tijd ) ) . '">' . esc_html( function_exists( 'dg_db_wanneer' ) ? dg_db_wanneer( $tijd ) : wp_date( 'j M', $tijd ) ) . '</time></a> · ' . dg_ber_zichtbaar_html( $p->ID )
		. ( dg_ber_flits_actief( $p->ID ) ? ' · <span class="dg-ber-flitslabel"><span class="dg-icoon dg-icoon-flits" aria-hidden="true"></span>Flits</span>' : '' );
	$h      = '<article class="dg-ber-kaart" id="bericht-' . (int) $p->ID . '" data-bericht="' . (int) $p->ID . '" data-openbaar="' . ( dg_ber_openbaar( $p->ID ) ? 1 : 0 ) . '">';
	$h     .= '<header>' . dg_ber_lid_html( (int) $p->post_author, $tijd, $extra ) . dg_ber_beheer_html( $p ) . '</header>';
	if ( $orig ) {
		$h .= '<p class="dg-ber-repostlabel"><span class="dg-icoon dg-icoon-repost" aria-hidden="true"></span>deelde dit bericht</p>';
	}
	if ( $p->post_title ) {
		$h .= $volledig ? '<h1 class="dg-ber-titel">' . esc_html( $p->post_title ) . '</h1>' : '<h2 class="dg-ber-titel"><a href="' . esc_url( $link ) . '">' . esc_html( $p->post_title ) . '</a></h2>';
	}
	if ( '' !== trim( $p->post_content ) ) {
		$h .= '<div class="dg-ber-tekst' . ( $volledig ? '' : ' is-ingeklapt' ) . '">' . dg_ber_tekst_html( $p->post_content ) . '</div>'
			. ( $volledig ? '' : '<button type="button" class="dg-ber-meer" hidden>…meer</button>' );
	}
	if ( $orig ) {
		$o  = get_post( $orig );
		$h .= $o && dg_ber_mag_lezen( $o ) ? dg_ber_ingebed( $o ) : '<div class="dg-ber-ingebed dg-ber-weg">Het oorspronkelijke bericht is er niet meer.</div>';
	} else {
		$h .= dg_ber_media_html( $p ) . dg_ber_link_html( $p );
	}
	$h .= dg_ber_tags_html( $p ) . dg_ber_telling_html( $p );
	if ( is_user_logged_in() ) {
		$h .= '<footer class="dg-ber-acties">' . dg_ber_reageerknop_html( $p )
			. '<a class="dg-ber-actie" href="' . esc_url( $link ) . '#opmerkingen"' . ( $volledig ? ' data-naar-opmerking' : '' ) . '><span class="dg-icoon dg-icoon-forum" aria-hidden="true"></span>Opmerking</a>'
			. '<button type="button" class="dg-ber-actie" data-repost="' . (int) ( $orig ?: $p->ID ) . '" data-openbaar="' . ( dg_ber_openbaar( $orig ?: $p->ID ) ? 1 : 0 ) . '"><span class="dg-icoon dg-icoon-repost" aria-hidden="true"></span>Reposten</button>'
			. '<button type="button" class="dg-ber-actie" data-sturen="' . (int) $p->ID . '"><span class="dg-icoon dg-icoon-verzenden" aria-hidden="true"></span>Verzenden</button>'
			. dg_ber_delen_html( $p ) . '</footer>';
	} else {
		$h .= '<footer class="dg-ber-acties">' . dg_ber_delen_html( $p ) . '</footer>';
	}
	return $h . '</article>';
}

/** Het oorspronkelijke bericht in een repost, klein. */
function dg_ber_ingebed( WP_Post $o ): string {
	$tijd = (int) get_post_time( 'U', true, $o );
	$link = get_permalink( $o );
	$h    = '<div class="dg-ber-ingebed">' . dg_ber_lid_html( (int) $o->post_author, $tijd, '<a href="' . esc_url( $link ) . '">' . esc_html( wp_date( 'j M Y', $tijd ) ) . '</a>' );
	if ( $o->post_title ) {
		$h .= '<h3 class="dg-ber-titel"><a href="' . esc_url( $link ) . '">' . esc_html( $o->post_title ) . '</a></h3>';
	}
	if ( '' !== trim( $o->post_content ) ) {
		$h .= '<div class="dg-ber-tekst is-ingeklapt">' . dg_ber_tekst_html( $o->post_content ) . '</div>';
	}
	return $h . dg_ber_media_html( $o ) . dg_ber_link_html( $o ) . '</div>';
}

// ---------------------------------------------------------------------------
// Opstellen
// ---------------------------------------------------------------------------

function dg_ber_taglijst(): string {
	$tags = get_terms( array( 'taxonomy' => DG_BER_TAG, 'hide_empty' => false, 'number' => 300, 'orderby' => 'count', 'order' => 'DESC' ) );
	$h    = '<datalist id="dg-ber-taglijst">';
	foreach ( is_array( $tags ) ? $tags : array() as $t ) {
		$h .= '<option value="' . esc_attr( $t->slug ) . '">' . esc_html( dg_ber_tagnaam( $t ) ) . '</option>';
	}
	return $h . '</datalist>';
}

/** Het formulier om een bericht te plaatsen (of te bewerken). */
function dg_ber_opstellen( ?WP_Post $p = null, ?WP_Term $tag = null ): string {
	if ( ! is_user_logged_in() ) {
		return '';
	}
	if ( ! get_option( 'dg_ber_vaste_tags' ) ) {
		dg_ber_vaste_tags();
	}
	$uid      = get_current_user_id();
	$tags     = $p ? implode( ', ', wp_get_object_terms( $p->ID, DG_BER_TAG, array( 'fields' => 'slugs' ) ) ) : ( $tag ? $tag->slug : '' );
	$openbaar = $p && dg_ber_openbaar( $p->ID );
	$flits    = $p && dg_ber_flits_actief( $p->ID );
	$tot      = $flits ? wp_date( 'Y-m-d', (int) get_post_meta( $p->ID, '_dg_flits_tot', true ) ) : '';
	$link     = $p ? (string) ( ( (array) get_post_meta( $p->ID, '_dg_link', true ) )['url'] ?? '' ) : '';
	$foto     = function_exists( 'bp_core_fetch_avatar' ) ? bp_core_fetch_avatar( array( 'item_id' => $uid, 'type' => 'thumb', 'width' => 48, 'height' => 48, 'html' => true, 'alt' => '' ) ) : get_avatar( $uid, 48 );
	$h        = '<form class="dg-ber-opstellen' . ( $p ? ' is-open' : '' ) . '" method="post" enctype="multipart/form-data" action="' . esc_url( admin_url( 'admin-post.php' ) ) . '" data-opstellen>'
		. wp_nonce_field( 'dg_bericht_opslaan', '_wpnonce', true, false )
		. '<input type="hidden" name="action" value="dg_bericht_opslaan">'
		. ( $p ? '<input type="hidden" name="id" value="' . (int) $p->ID . '">' : '' )
		. '<div class="dg-ber-opstellen-kop">' . $foto
		. '<label class="screen-reader-text" for="dg-ber-tekst">Je bericht</label>'
		. '<textarea id="dg-ber-tekst" name="tekst" rows="' . ( $p ? 6 : 2 ) . '" maxlength="' . DG_BER_MAX_TEKST . '" placeholder="Deel een update, een vraag of een mooi resultaat…">' . esc_textarea( $p ? $p->post_content : '' ) . '</textarea></div>'
		. '<div class="dg-ber-opstellen-rest">'
		. '<label>Titel <small>(optioneel; nodig voor een nieuwsflits)</small><input type="text" name="titel" maxlength="140" value="' . esc_attr( $p ? $p->post_title : '' ) . '"></label>'
		. '<label>Link <small>(optioneel: een artikel, video of document; anders de eerste link in je tekst)</small><input type="url" name="link" inputmode="url" placeholder="https://…" value="' . esc_attr( $link ) . '"></label>'
		. '<label>Tags <small>(gescheiden door komma\'s; ook #woord in je tekst wordt een tag)</small><input type="text" name="tags" list="dg-ber-taglijst" autocomplete="off" placeholder="bijvoorbeeld xena, records-management" value="' . esc_attr( $tags ) . '"></label>'
		. dg_ber_taglijst();
	if ( $p ) {
		$media = array_filter( array_map( 'intval', (array) get_post_meta( $p->ID, '_dg_media', true ) ) );
		if ( $media ) {
			$h .= '<fieldset class="dg-ber-huidig"><legend>Foto\'s en video</legend>';
			foreach ( $media as $m ) {
				$h .= '<label>' . ( wp_attachment_is_image( $m ) ? wp_get_attachment_image( $m, 'thumbnail' ) : '<span class="dg-icoon dg-icoon-video" aria-hidden="true"></span>' . esc_html( basename( (string) get_attached_file( $m ) ) ) )
					. '<span><input type="checkbox" name="weg[]" value="' . (int) $m . '"> verwijderen</span></label>';
			}
			$h .= '</fieldset>';
		}
	}
	$h .= '<div class="dg-ber-bestanden" data-bestanden></div>'
		. '<div class="dg-ber-opties">'
		. '<label class="dg-ber-knopje"><span class="dg-icoon dg-icoon-foto" aria-hidden="true"></span>Foto\'s<input type="file" name="fotos[]" accept="image/jpeg,image/png,image/webp,image/gif" multiple class="screen-reader-text"></label>'
		. '<label class="dg-ber-knopje"><span class="dg-icoon dg-icoon-video" aria-hidden="true"></span>Video<input type="file" name="video" accept="video/mp4,video/webm" class="screen-reader-text"></label>'
		. '<label class="dg-ber-zichtkeuze"><span class="screen-reader-text">Wie mag het lezen</span><select name="zichtbaar">'
		. '<option value="leden"' . selected( ! $openbaar, true, false ) . '>🔒 Alleen leden</option>'
		. '<option value="openbaar"' . selected( $openbaar, true, false ) . '>🌐 Openbaar</option></select></label>'
		. '<label class="dg-ber-flitskeuze"><input type="checkbox" name="flits" value="1"' . checked( $flits, true, false ) . '> Ook als <strong>nieuwsflits</strong> op de homepage</label>'
		. '<label class="dg-ber-flitstot"' . ( $flits ? '' : ' hidden' ) . '>tot en met <input type="date" name="flits_tot" value="' . esc_attr( $tot ) . '" min="' . esc_attr( wp_date( 'Y-m-d' ) ) . '"> <small>(leeg: ' . DG_BER_FLITS_DAGEN . ' dagen)</small></label>'
		. '<button type="submit" class="dg-db-knop">' . ( $p ? 'Opslaan' : 'Plaatsen' ) . '</button>'
		. ( $p ? '<a href="' . esc_url( get_permalink( $p ) ) . '">Annuleren</a>' : '' )
		. '</div>'
		. '<p class="dg-ber-klein"><strong>Openbaar</strong>: iedereen kan het lezen en delen, ook op LinkedIn. <strong>Alleen leden</strong>: alleen wie is ingelogd. Geen persoonsgegevens of vertrouwelijke stukken. Foto\'s: hooguit ' . DG_BER_MAX_FOTOS . ', de locatie gaat eruit. Video: hooguit ' . DG_BER_VIDEO_MB . ' MB; een video van je telefoon kan de plek bevatten waar hij is opgenomen. Langer? Plaats hem op YouTube, Vimeo of PeerTube en plak de link.</p>'
		. '</div></form>';
	return $h;
}

// ---------------------------------------------------------------------------
// Opmerkingen
// ---------------------------------------------------------------------------

function dg_ber_opmerkingen( WP_Post $p ): string {
	$aantal = (int) get_comments_number( $p );
	$h      = '<section class="dg-ber-opmerkingen" id="opmerkingen"><h2>' . ( $aantal ? $aantal . ( 1 === $aantal ? ' opmerking' : ' opmerkingen' ) : 'Opmerkingen' ) . '</h2>';
	if ( ! is_user_logged_in() ) {
		return $h . '<p>' . ( $aantal ? 'De opmerkingen zijn alleen voor leden. ' : '' ) . '<a href="' . esc_url( wp_login_url( get_permalink( $p ) ) ) . '">Log in</a> om te reageren.</p></section>';
	}
	$alle   = get_comments( array( 'post_id' => $p->ID, 'status' => 'approve', 'order' => 'ASC' ) );
	$kinder = array();
	$top    = array();
	$ids    = wp_list_pluck( $alle, 'comment_ID' );
	foreach ( $alle as $c ) {
		$ouder = (int) $c->comment_parent;
		// Eén niveau diep: een antwoord op een antwoord hangt onder de eerste opmerking.
		while ( $ouder && in_array( (string) $ouder, $ids, true ) && (int) get_comment( $ouder )->comment_parent ) {
			$ouder = (int) get_comment( $ouder )->comment_parent;
		}
		if ( $ouder ) {
			$kinder[ $ouder ][] = $c;
		} else {
			$top[] = $c;
		}
	}
	$een = static function ( WP_Comment $c, bool $antwoord ) use ( $p ): string {
		$tijd = strtotime( $c->comment_date_gmt . ' UTC' );
		return '<li class="dg-ber-opmerking" id="opmerking-' . (int) $c->comment_ID . '">'
			. dg_ber_lid_html( (int) $c->user_id, $tijd, esc_html( function_exists( 'dg_db_wanneer' ) ? dg_db_wanneer( $tijd ) : wp_date( 'j M', $tijd ) ) )
			. '<div class="dg-ber-opmerking-tekst">' . dg_ber_tekst_html( $c->comment_content ) . '</div>'
			. '<button type="button" class="dg-ber-antwoord" data-antwoord="' . (int) ( $antwoord ? $c->comment_parent : $c->comment_ID ) . '" data-naam="' . esc_attr( $c->comment_author ) . '">Antwoorden</button>';
	};
	if ( $top ) {
		$h .= '<ol class="dg-ber-opmerkinglijst">';
		foreach ( $top as $c ) {
			$h .= $een( $c, false );
			if ( ! empty( $kinder[ (int) $c->comment_ID ] ) ) {
				$h .= '<ol>' . implode( '', array_map( static fn( $k ) => $een( $k, true ) . '</li>', $kinder[ (int) $c->comment_ID ] ) ) . '</ol>';
			}
			$h .= '</li>';
		}
		$h .= '</ol>';
	}
	ob_start();
	wp_comment_form_unfiltered_html_nonce();
	$nonce = (string) ob_get_clean();
	$uid   = get_current_user_id();
	$foto  = function_exists( 'bp_core_fetch_avatar' ) ? bp_core_fetch_avatar( array( 'item_id' => $uid, 'type' => 'thumb', 'width' => 40, 'height' => 40, 'html' => true, 'alt' => '' ) ) : get_avatar( $uid, 40 );
	$h    .= '<form class="dg-ber-opmerkingform" method="post" action="' . esc_url( site_url( '/wp-comments-post.php' ) ) . '">' . $foto
		. '<div><p class="dg-ber-antwoord-op" hidden>Antwoord aan <strong></strong> <button type="button" data-geen-antwoord>annuleren</button></p>'
		. '<label class="screen-reader-text" for="dg-ber-opmerking">Je opmerking</label>'
		. '<textarea id="dg-ber-opmerking" name="comment" rows="2" required maxlength="3000" placeholder="Schrijf een opmerking…"></textarea>'
		. '<input type="hidden" name="comment_post_ID" value="' . (int) $p->ID . '"><input type="hidden" name="comment_parent" value="0">' . $nonce
		. '<button type="submit" class="dg-db-knop">Plaatsen</button></div></form>';
	return $h . '</section>';
}

// ---------------------------------------------------------------------------
// Nieuwsflitsen
// ---------------------------------------------------------------------------

/** De lopende flitsen, nieuwste eerst (gasten: alleen openbare). */
function dg_ber_flitsen( int $max = 3 ): array {
	return get_posts(
		array(
			'post_type'        => DG_BER_TYPE,
			'post_status'      => 'publish',
			'posts_per_page'   => $max,
			'meta_query'       => array( array( 'key' => '_dg_flits_tot', 'value' => time(), 'compare' => '>', 'type' => 'NUMERIC' ) ), // phpcs:ignore WordPress.DB.SlowDBQuery
			'orderby'          => 'date',
			'order'            => 'DESC',
			'suppress_filters' => false,
		)
	);
}

function dg_ber_flitsen_html( int $max = 3, string $plek = 'home' ): string {
	$flitsen = dg_ber_flitsen( $max );
	if ( ! $flitsen ) {
		return '';
	}
	$h = '<section class="dg-flitsen dg-flitsen-' . esc_attr( $plek ) . '" aria-labelledby="dg-flitsen-kop-' . esc_attr( $plek ) . '"><header><h2 id="dg-flitsen-kop-' . esc_attr( $plek ) . '"><span class="dg-icoon dg-icoon-flits" aria-hidden="true"></span>Nieuwsflitsen</h2>'
		. '<a href="' . esc_url( get_post_type_archive_link( DG_BER_TYPE ) ) . '">Alle nieuws ›</a></header><ul>';
	foreach ( $flitsen as $f ) {
		$beeld = get_the_post_thumbnail( $f, 'thumbnail', array( 'loading' => 'lazy', 'alt' => '' ) );
		$h    .= '<li><a href="' . esc_url( get_permalink( $f ) ) . '">' . ( $beeld ?: '<span class="dg-flits-geenbeeld"><span class="dg-icoon dg-icoon-nieuws" aria-hidden="true"></span></span>' )
			. '<span><strong>' . esc_html( $f->post_title ) . '</strong><span class="dg-flits-tekst">' . esc_html( wp_trim_words( $f->post_content, 40 ) ) . '</span>'
			. '<small>' . esc_html( wp_date( 'j F Y', (int) get_post_time( 'U', true, $f ) ) ) . '</small></span></a></li>';
	}
	return $h . '</ul></section>';
}

add_shortcode( 'dg_flitsen', static fn( $attr ) => dg_ber_flitsen_html( max( 1, min( 6, (int) ( $attr['aantal'] ?? 3 ) ) ) ) );

// ---------------------------------------------------------------------------
// De pagina's: /nieuws/, /nieuws/tag/<tag>/, /nieuws/<bericht>/
// ---------------------------------------------------------------------------

function dg_ber_is_pagina(): bool {
	return is_post_type_archive( DG_BER_TYPE ) || is_tax( DG_BER_TAG ) || is_singular( DG_BER_TYPE );
}

add_action(
	'pre_get_posts',
	static function ( $q ) {
		if ( ! is_admin() && $q->is_main_query() && ( $q->is_post_type_archive( DG_BER_TYPE ) || $q->is_tax( DG_BER_TAG ) ) ) {
			$q->set( 'posts_per_page', DG_BER_PER_PAGINA );
			$q->set( 'post_type', DG_BER_TYPE );
		}
	}
);

add_filter(
	'document_title_parts',
	static function ( $delen ) {
		if ( is_post_type_archive( DG_BER_TYPE ) ) {
			$delen['title'] = 'Nieuws';
		} elseif ( is_tax( DG_BER_TAG ) ) {
			$delen['title'] = '#' . dg_ber_tagnaam( get_queried_object() ) . ' · Nieuws';
		} elseif ( is_singular( DG_BER_TYPE ) ) {
			$p              = get_queried_object();
			$delen['title'] = $p->post_title ?: wp_trim_words( $p->post_content, 10 );
		}
		return $delen;
	}
);

// Een bericht zonder titel heet elders (zoeken, RSS, beheer) naar het begin van de tekst.
add_filter(
	'the_title',
	static function ( $titel, $id = 0 ) {
		if ( '' !== (string) $titel || ! $id || DG_BER_TYPE !== get_post_type( $id ) ) {
			return $titel;
		}
		return wp_trim_words( (string) get_post_field( 'post_content', $id ), 10 );
	},
	10,
	2
);

add_filter( 'body_class', static fn( $k ) => dg_ber_is_pagina() ? array_merge( $k, array( 'dg-berichten-pagina' ) ) : $k );

// Het kaartje bij het delen (LinkedIn, Bluesky, WhatsApp): alleen voor openbare berichten.
add_action(
	'wp_head',
	static function () {
		if ( ! is_singular( DG_BER_TYPE ) ) {
			return;
		}
		$p = get_queried_object();
		if ( ! dg_ber_openbaar( $p->ID ) ) {
			echo '<meta name="robots" content="noindex">' . "\n";
			return;
		}
		$titel = $p->post_title ?: wp_trim_words( $p->post_content, 12 );
		$tekst = wp_trim_words( $p->post_content, 35 );
		$beeld = get_the_post_thumbnail_url( $p, 'large' );
		$auteur = get_userdata( (int) $p->post_author );
		$meta  = array(
			'og:type'        => 'article',
			'og:site_name'   => 'Digitaal GemeenGoed',
			'og:title'       => $titel,
			'og:description' => $tekst,
			'og:url'         => get_permalink( $p ),
			'og:image'       => $beeld ?: '',
			'article:author' => $auteur ? $auteur->display_name : '',
			'twitter:card'   => $beeld ? 'summary_large_image' : 'summary',
		);
		foreach ( array_filter( $meta ) as $k => $v ) {
			printf( '<meta %s="%s" content="%s">' . "\n", str_starts_with( $k, 'twitter:' ) ? 'name' : 'property', esc_attr( $k ), esc_attr( $v ) );
		}
		echo '<meta name="description" content="' . esc_attr( $tekst ) . '">' . "\n";
	},
	5
);

add_action(
	'wp_enqueue_scripts',
	static function () {
		$versie = wp_get_theme()->get( 'Version' ) . '.' . DG_BER_VERSIE;
		wp_enqueue_style( 'dg-berichten', get_stylesheet_directory_uri() . '/berichten.css', array(), $versie );
		if ( dg_ber_is_pagina() ) {
			wp_enqueue_script( 'dg-berichten', get_stylesheet_directory_uri() . '/berichten.js', array(), $versie, array( 'in_footer' => true, 'strategy' => 'defer' ) );
			wp_add_inline_script(
				'dg-berichten',
				'window.dgBerichten = ' . wp_json_encode(
					array(
						'rest'  => esc_url_raw( rest_url( 'dg/v1/' ) ),
						'nonce' => is_user_logged_in() ? wp_create_nonce( 'wp_rest' ) : '',
					)
				) . ';',
				'before'
			);
		}
	},
	30
);

add_action(
	'template_redirect',
	static function () {
		if ( ! dg_ber_is_pagina() || is_feed() ) {
			return;
		}
		if ( is_singular( DG_BER_TYPE ) && ! dg_ber_mag_lezen( get_queried_object() ) ) {
			auth_redirect();
		}
		remove_action( 'buddyx_sub_header', 'buddyx_sub_header' );
		get_header();
		echo dg_ber_pagina(); // phpcs:ignore WordPress.Security.EscapeOutput -- opgebouwd met esc_*.
		get_footer();
		exit;
	},
	20
);

function dg_ber_pagina(): string {
	global $wp_query;
	$h = '<div class="dg-ber-pagina"><main class="dg-ber-hoofd" id="dg-berichten">' . dg_ber_melding();
	if ( is_singular( DG_BER_TYPE ) ) {
		$p = get_queried_object();
		$h .= '<p class="dg-ber-terug"><a href="' . esc_url( get_post_type_archive_link( DG_BER_TYPE ) ) . '">‹ Alle nieuws</a></p>';
		// phpcs:ignore WordPress.Security.NonceVerification.Recommended -- alleen tonen.
		if ( ! empty( $_GET['bewerken'] ) && (int) $p->post_author === get_current_user_id() ) {
			$h .= '<h1 class="dg-ber-kop">Bericht bewerken</h1>' . dg_ber_opstellen( $p );
		} else {
			$h .= dg_ber_kaart( $p, true ) . dg_ber_opmerkingen( $p );
		}
	} else {
		$tag = is_tax( DG_BER_TAG ) ? get_queried_object() : null;
		if ( $tag ) {
			$links = dg_ber_tag_links( $tag );
			$h    .= '<header class="dg-ber-tagkop"><small>' . esc_html( dg_ber_soortnaam( $tag ) ) . '</small><h1>#' . esc_html( dg_ber_tagnaam( $tag ) ) . '</h1>'
				. ( $tag->description ? '<p>' . esc_html( $tag->description ) . '</p>' : '' )
				. ( $links ? '<p class="dg-ber-taglinks">' . implode( '', array_map( static fn( $l ) => '<a href="' . esc_url( $l[2] ) . '"><span class="dg-icoon dg-icoon-' . esc_attr( $l[0] ) . '" aria-hidden="true"></span>' . esc_html( $l[1] ) . '</a>', $links ) ) . '</p>' : '' )
				. '<a class="dg-ber-alles" href="' . esc_url( get_post_type_archive_link( DG_BER_TYPE ) ) . '">‹ Alle nieuws</a></header>';
		} else {
			$h .= '<header class="dg-ber-feedkop"><h1>Nieuws</h1><p>Updates, vragen en resultaten van leden, projecten en werkgroepen.</p></header>';
		}
		if ( is_user_logged_in() ) {
			$h .= dg_ber_opstellen( null, $tag );
		} else {
			$h .= '<div class="dg-ber-gast"><p>Je ziet de openbare berichten. Leden zien ook de berichten voor leden, en kunnen zelf iets plaatsen en reageren.</p><p><a class="dg-db-knop" href="' . esc_url( wp_login_url( (string) ( $_SERVER['REQUEST_URI'] ?? '' ) ) ) . '">Aanmelden</a> <a class="dg-db-knop dg-db-knop-rand" href="' . esc_url( home_url( '/lid-worden/' ) ) . '">Lid worden</a></p></div>'; // phpcs:ignore WordPress.Security.ValidatedSanitizedInput
		}
		if ( have_posts() ) {
			dg_ber_reacties( wp_list_pluck( $wp_query->posts, 'ID' ) );
			$h .= '<div class="dg-ber-feed">';
			foreach ( $wp_query->posts as $p ) {
				$h .= dg_ber_kaart( $p );
			}
			$h .= '</div>';
			$pag = paginate_links(
				array(
					'total'     => (int) $wp_query->max_num_pages,
					'current'   => max( 1, (int) get_query_var( 'paged' ) ),
					'prev_text' => '‹ Nieuwer',
					'next_text' => 'Ouder ›',
				)
			);
			$h  .= $pag ? '<nav class="dg-ber-paginas" aria-label="Pagina\'s">' . $pag . '</nav>' : '';
		} else {
			$h .= '<p class="dg-ber-leeg">' . ( $tag ? 'Nog geen berichten met deze tag.' : 'Nog geen berichten.' ) . ( is_user_logged_in() ? ' Plaats het eerste!' : '' ) . '</p>';
		}
	}
	$h .= '</main><aside class="dg-ber-zij">' . dg_ber_flitsen_html( 3, 'zij' ) . dg_ber_onderwerpen() . '</aside></div>';
	if ( is_user_logged_in() ) {
		$h .= dg_ber_dialogen();
	}
	return $h;
}

/** De tags in de zijbalk: de coöperatie, werkgroepen, projecten en de populairste vrije tags. */
function dg_ber_onderwerpen(): string {
	$alle   = get_terms( array( 'taxonomy' => DG_BER_TAG, 'hide_empty' => false, 'number' => 400 ) );
	$groepen = array( 'cooperatie' => array(), 'werkgroep' => array(), 'project' => array(), '' => array() );
	foreach ( is_array( $alle ) ? $alle : array() as $t ) {
		$s = (string) get_term_meta( $t->term_id, 'dg_soort', true );
		if ( '' === $s && ! $t->count ) {
			continue;
		}
		$groepen[ isset( $groepen[ $s ] ) ? $s : '' ][] = $t;
	}
	usort( $groepen[''], static fn( $a, $b ) => $b->count <=> $a->count );
	$groepen[''] = array_slice( $groepen[''], 0, 12 );
	$titels      = array( 'cooperatie' => '', 'werkgroep' => 'Werkgroepen', 'project' => 'Projecten', '' => 'Populaire tags' );
	$huidig      = is_tax( DG_BER_TAG ) ? get_queried_object_id() : 0;
	$h           = '<section class="dg-ber-onderwerpen"><h2><span class="dg-icoon dg-icoon-nieuws" aria-hidden="true"></span>Onderwerpen</h2>';
	foreach ( $groepen as $s => $tags ) {
		if ( ! $tags ) {
			continue;
		}
		$h .= ( $titels[ $s ] ? '<h3>' . esc_html( $titels[ $s ] ) . '</h3>' : '' ) . '<ul>';
		foreach ( $tags as $t ) {
			$h .= '<li><a class="dg-ber-tag' . ( $s ? ' is-vast' : '' ) . ( $huidig === $t->term_id ? ' is-huidig' : '' ) . '" href="' . esc_url( get_term_link( $t ) ) . '">#' . esc_html( dg_ber_tagnaam( $t ) ) . '</a></li>';
		}
		$h .= '</ul>';
	}
	return $h . '</section>';
}

/** De vensters voor reposten, verzenden en melden (één keer per pagina). */
function dg_ber_dialogen(): string {
	return '<dialog class="dg-ber-dialoog" id="dg-ber-repost" aria-labelledby="dg-ber-repost-kop"><form method="post" action="' . esc_url( admin_url( 'admin-post.php' ) ) . '">'
		. wp_nonce_field( 'dg_bericht_opslaan', '_wpnonce', true, false )
		. '<input type="hidden" name="action" value="dg_bericht_opslaan"><input type="hidden" name="repost_van" value="">'
		. '<h2 id="dg-ber-repost-kop">Reposten</h2><p>Het bericht komt in de feed, met jouw naam erbij. Zet er eventueel iets bij.</p>'
		. '<label class="screen-reader-text" for="dg-ber-repost-tekst">Je toelichting</label><textarea id="dg-ber-repost-tekst" name="tekst" rows="3" maxlength="' . DG_BER_MAX_TEKST . '" placeholder="Waarom is dit interessant? (optioneel)"></textarea>'
		. '<label>Wie mag het lezen <select name="zichtbaar"><option value="leden">🔒 Alleen leden</option><option value="openbaar">🌐 Openbaar</option></select></label>'
		. '<p class="dg-ber-klein" data-alleen-leden hidden>Het oorspronkelijke bericht is alleen voor leden, dus je repost ook.</p>'
		. '<div class="dg-ber-dialoogknoppen"><button type="button" class="dg-db-knop dg-db-knop-rand" data-sluit>Annuleren</button><button type="submit" class="dg-db-knop">Reposten</button></div></form></dialog>'

		. '<dialog class="dg-ber-dialoog" id="dg-ber-sturen" aria-labelledby="dg-ber-sturen-kop"><form>'
		. '<h2 id="dg-ber-sturen-kop">Verzenden als privébericht</h2>'
		. '<label>Aan <small>(typ een naam; hooguit 10 leden)</small><input type="text" data-zoek-lid autocomplete="off" list="dg-ber-ledenlijst" placeholder="Naam van een lid"></label><datalist id="dg-ber-ledenlijst"></datalist>'
		. '<ul class="dg-ber-ontvangers" data-ontvangers></ul>'
		. '<label>Bericht <small>(optioneel)</small><textarea name="tekst" rows="3" maxlength="1000" placeholder="Dit is misschien iets voor jou…"></textarea></label>'
		. '<p class="dg-ber-uitslag" role="status" data-uitslag></p>'
		. '<div class="dg-ber-dialoogknoppen"><button type="button" class="dg-db-knop dg-db-knop-rand" data-sluit>Sluiten</button><button type="submit" class="dg-db-knop">Versturen</button></div></form></dialog>'

		. '<dialog class="dg-ber-dialoog" id="dg-ber-melden" aria-labelledby="dg-ber-melden-kop"><form>'
		. '<h2 id="dg-ber-melden-kop">Bericht melden</h2><p>Staan er persoonsgegevens of vertrouwelijke informatie in, of past het niet bij de community? De kerngroep krijgt je melding en kijkt ernaar.</p>'
		. '<label>Waarom? <textarea name="reden" rows="3" maxlength="1000"></textarea></label>'
		. '<p class="dg-ber-uitslag" role="status" data-uitslag></p>'
		. '<div class="dg-ber-dialoogknoppen"><button type="button" class="dg-db-knop dg-db-knop-rand" data-sluit>Sluiten</button><button type="submit" class="dg-db-knop">Melden</button></div></form></dialog>';
}
